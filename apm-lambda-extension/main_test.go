package main

import (
	"bytes"
	e2e_testing "elastic/apm-lambda-extension/e2e-testing"
	"elastic/apm-lambda-extension/extension"
	"elastic/apm-lambda-extension/logsapi"
	json "encoding/json"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

type RegistrationResponse struct {
	FunctionName    string `json:"functionName"`
	FunctionVersion string `json:"functionVersion"`
	Handler         string `json:"handler"`
}

func initMockServers(eventsChannel chan MockEvent) (*httptest.Server, *httptest.Server, chan struct{}) {

	// Mock APM Server
	hangChan := make(chan struct{})
	APMServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/intake/v2/events" {
			decompressedBytes, err := e2e_testing.GetDecompressedBytesFromRequest(r)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
			switch APMServerBehavior(decompressedBytes) {
			case TimelyResponse:
				w.WriteHeader(http.StatusAccepted)
			case SlowResponse:
				time.Sleep(2 * time.Second)
				w.WriteHeader(http.StatusAccepted)
			case Hangs:
				<-hangChan
			case Crashes:
				panic("Server crashed")
			default:
				w.WriteHeader(http.StatusInternalServerError)
			}
		}
	}))
	os.Setenv("ELASTIC_APM_LAMBDA_APM_SERVER", APMServer.URL)
	os.Setenv("ELASTIC_APM_SECRET_TOKEN", "none")

	// Mock Lambda Server
	lambdaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.RequestURI {
		// Extension registration request
		case "/2020-01-01/extension/register":
			w.Header().Set("Lambda-Extension-Identifier", "b03a29ec-ee63-44cd-8e53-3987a8e8aa8e")
			body, _ := json.Marshal(RegistrationResponse{
				FunctionName:    "UnitTestingMockLambda",
				FunctionVersion: "$LATEST",
				Handler:         "main_test.mock_lambda",
			})
			w.Write(body)
		case "/2020-01-01/extension/event/next":
			currId := uuid.New().String()
			select {
			case nextEvent := <-eventsChannel:
				w.Write(sendNextEventInfo(currId, nextEvent))
				go processMockEvent(currId, nextEvent, APMServer)
			default:
				finalShutDown := MockEvent{
					Type:              Shutdown,
					ExecutionDuration: 0,
					Timeout:           0,
				}
				w.Write(sendNextEventInfo(currId, finalShutDown))
				go processMockEvent(currId, finalShutDown, APMServer)
			}
		// Logs API subscription request
		case "/2020-08-15/logs":
			w.WriteHeader(http.StatusOK)
			os.Setenv("ELASTIC_APM_LAMBDA_LOGS_LISTENER_ADDRESS", "localhost:8205")
		}
	}))

	slicedLambdaURL := strings.Split(lambdaServer.URL, "//")
	strippedLambdaURL := slicedLambdaURL[1]
	os.Setenv("AWS_LAMBDA_RUNTIME_API", strippedLambdaURL)
	extensionClient = extension.NewClient(os.Getenv("AWS_LAMBDA_RUNTIME_API"))

	return lambdaServer, APMServer, hangChan
}

type MockEventType string

const (
	InvokeHang                         MockEventType = "Hang"
	InvokeStandard                     MockEventType = "Standard"
	InvokeStandardFlush                MockEventType = "Flush"
	InvokeWaitgroupsRace               MockEventType = "InvokeWaitgroupsRace"
	InvokeMultipleTransactionsOverload MockEventType = "MultipleTransactionsOverload"
	Shutdown                           MockEventType = "Shutdown"
)

type APMServerBehavior string

const (
	TimelyResponse APMServerBehavior = "TimelyResponse"
	SlowResponse   APMServerBehavior = "SlowResponse"
	Hangs          APMServerBehavior = "Hangs"
	Crashes        APMServerBehavior = "Crashes"
)

type MockEvent struct {
	Type              MockEventType
	APMServerBehavior APMServerBehavior
	ExecutionDuration float64
	Timeout           float64
}

func processMockEvent(currId string, event MockEvent, APMServer *httptest.Server) {
	sendLogEvent(currId, "platform.start")
	client := http.Client{}
	switch event.Type {
	case InvokeHang:
		time.Sleep(time.Duration(event.Timeout) * time.Second)
	case InvokeStandard:
		time.Sleep(time.Duration(event.ExecutionDuration) * time.Second)
		req, _ := http.NewRequest("POST", "http://localhost:8200/intake/v2/events", bytes.NewBuffer([]byte(event.APMServerBehavior)))
		client.Do(req)
	case InvokeStandardFlush:
		time.Sleep(time.Duration(event.ExecutionDuration) * time.Second)
		reqData, _ := http.NewRequest("POST", "http://localhost:8200/intake/v2/events?flushed=true", bytes.NewBuffer([]byte(event.APMServerBehavior)))
		client.Do(reqData)
	case InvokeWaitgroupsRace:
		time.Sleep(time.Duration(event.ExecutionDuration) * time.Second)
		reqData0, _ := http.NewRequest("POST", "http://localhost:8200/intake/v2/events", bytes.NewBuffer([]byte(event.APMServerBehavior)))
		reqData1, _ := http.NewRequest("POST", "http://localhost:8200/intake/v2/events", bytes.NewBuffer([]byte(event.APMServerBehavior)))
		go client.Do(reqData0)
		go client.Do(reqData1)
		time.Sleep(650 * time.Microsecond)
	case InvokeMultipleTransactionsOverload:
		wg := sync.WaitGroup{}
		for i := 0; i < 200; i++ {
			go func() {
				wg.Add(1)
				time.Sleep(time.Duration(event.ExecutionDuration) * time.Second)
				reqData, _ := http.NewRequest("POST", "http://localhost:8200/intake/v2/events", bytes.NewBuffer([]byte(event.APMServerBehavior)))
				client.Do(reqData)
				wg.Done()
			}()
		}
		wg.Wait()
	case Shutdown:
		reqData, _ := http.NewRequest("POST", "http://localhost:8200/intake/v2/events", bytes.NewBuffer([]byte(event.APMServerBehavior)))
		client.Do(reqData)
	}
	sendLogEvent(currId, "platform.runtimeDone")
}

func sendNextEventInfo(id string, event MockEvent) []byte {
	nextEventInfo := extension.NextEventResponse{
		EventType:          "INVOKE",
		DeadlineMs:         time.Now().UnixNano()/int64(time.Millisecond) + int64(event.Timeout*1000),
		RequestID:          id,
		InvokedFunctionArn: "arn:aws:lambda:eu-central-1:627286350134:function:main_unit_test",
		Tracing:            extension.Tracing{},
	}
	if event.Type == Shutdown {
		nextEventInfo.EventType = "SHUTDOWN"
	}

	out, _ := json.Marshal(nextEventInfo)
	return out
}

func sendLogEvent(requestId string, logEventType string) {
	record := logsapi.LogEventRecord{
		RequestId: requestId,
	}
	logEvent := logsapi.LogEvent{
		Time:   time.Now(),
		Type:   logEventType,
		Record: record,
	}
	logEvent.RawRecord, _ = json.Marshal(logEvent.Record)
	body, _ := json.Marshal([]logsapi.LogEvent{logEvent})
	req, _ := http.NewRequest("POST", "http://localhost:8205", bytes.NewBuffer(body))
	client := http.Client{}
	client.Do(req)
}

func eventQueueGenerator(inputQueue []MockEvent, eventsChannel chan MockEvent) {
	for _, event := range inputQueue {
		eventsChannel <- event
	}
}

// TESTS

func TestStandardEventsChain(t *testing.T) {
	extension.Log = extension.InitLogger()
	http.DefaultServeMux = new(http.ServeMux)

	eventsChannel := make(chan MockEvent, 100)
	defer close(eventsChannel)
	lambdaServer, APMServer, _ := initMockServers(eventsChannel)
	defer lambdaServer.Close()
	defer APMServer.Close()

	eventsChain := []MockEvent{
		{Type: InvokeStandard, APMServerBehavior: TimelyResponse, ExecutionDuration: 1, Timeout: 5},
	}
	eventQueueGenerator(eventsChain, eventsChannel)
	main()
}

func TestAPMServerDown(t *testing.T) {
	extension.Log = extension.InitLogger()
	http.DefaultServeMux = new(http.ServeMux)

	eventsChannel := make(chan MockEvent, 100)
	defer close(eventsChannel)
	lambdaServer, APMServer, _ := initMockServers(eventsChannel)
	defer lambdaServer.Close()
	APMServer.Close()

	eventsChain := []MockEvent{
		{Type: InvokeStandard, APMServerBehavior: TimelyResponse, ExecutionDuration: 1, Timeout: 5},
	}
	eventQueueGenerator(eventsChain, eventsChannel)
	main()
}

func TestAPMServerHangs(t *testing.T) {
	extension.Log = extension.InitLogger()
	http.DefaultServeMux = new(http.ServeMux)

	eventsChannel := make(chan MockEvent, 100)
	defer close(eventsChannel)
	lambdaServer, APMServer, hangChan := initMockServers(eventsChannel)
	defer lambdaServer.Close()
	defer APMServer.Close()

	eventsChain := []MockEvent{
		{Type: InvokeStandard, APMServerBehavior: Hangs, ExecutionDuration: 1, Timeout: 5},
	}
	eventQueueGenerator(eventsChain, eventsChannel)
	main()
	hangChan <- struct{}{}
}

func TestAPMServerCrashesDuringExecution(t *testing.T) {
	extension.Log = extension.InitLogger()
	http.DefaultServeMux = new(http.ServeMux)

	eventsChannel := make(chan MockEvent, 100)
	defer close(eventsChannel)
	lambdaServer, APMServer, _ := initMockServers(eventsChannel)
	defer lambdaServer.Close()
	defer APMServer.Close()

	eventsChain := []MockEvent{
		{Type: InvokeStandard, APMServerBehavior: Crashes, ExecutionDuration: 1, Timeout: 5},
	}
	eventQueueGenerator(eventsChain, eventsChannel)
	main()
}

func TestFullChannel(t *testing.T) {
	extension.Log = extension.InitLogger()
	http.DefaultServeMux = new(http.ServeMux)

	eventsChannel := make(chan MockEvent, 1000)
	defer close(eventsChannel)
	lambdaServer, APMServer, _ := initMockServers(eventsChannel)
	defer lambdaServer.Close()
	defer APMServer.Close()

	eventsChain := []MockEvent{
		{Type: InvokeMultipleTransactionsOverload, APMServerBehavior: TimelyResponse, ExecutionDuration: 0.01, Timeout: 5},
	}
	eventQueueGenerator(eventsChain, eventsChannel)
	main()
}

func TestFullChannelSlowAPMServer(t *testing.T) {
	extension.Log = extension.InitLogger()
	http.DefaultServeMux = new(http.ServeMux)

	os.Setenv("ELASTIC_APM_SEND_STRATEGY", "background")
	eventsChannel := make(chan MockEvent, 1000)
	defer close(eventsChannel)
	lambdaServer, APMServer, _ := initMockServers(eventsChannel)
	defer lambdaServer.Close()
	defer APMServer.Close()

	eventsChain := []MockEvent{
		{Type: InvokeMultipleTransactionsOverload, APMServerBehavior: SlowResponse, ExecutionDuration: 0.01, Timeout: 5},
	}
	eventQueueGenerator(eventsChain, eventsChannel)
	main()
}

// Error parsing the data

func TestFlush(t *testing.T) {
	extension.Log = extension.InitLogger()
	http.DefaultServeMux = new(http.ServeMux)

	eventsChannel := make(chan MockEvent, 100)
	//defer close(eventsChannel)
	lambdaServer, APMServer, _ := initMockServers(eventsChannel)
	defer lambdaServer.Close()
	defer APMServer.Close()

	eventsChain := []MockEvent{
		{Type: InvokeStandardFlush, APMServerBehavior: TimelyResponse, ExecutionDuration: 1, Timeout: 5},
	}
	eventQueueGenerator(eventsChain, eventsChannel)
	main()
}

func TestWaitGroup(t *testing.T) {
	extension.Log = extension.InitLogger()
	http.DefaultServeMux = new(http.ServeMux)

	eventsChannel := make(chan MockEvent, 100)
	defer close(eventsChannel)
	lambdaServer, APMServer, _ := initMockServers(eventsChannel)
	defer lambdaServer.Close()
	defer APMServer.Close()

	eventsChain := []MockEvent{
		{Type: InvokeWaitgroupsRace, APMServerBehavior: TimelyResponse, ExecutionDuration: 0.1, Timeout: 500},
	}
	eventQueueGenerator(eventsChain, eventsChannel)
	main()
}
