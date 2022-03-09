package main

import (
	"bytes"
	e2e_testing "elastic/apm-lambda-extension/e2e-testing"
	"elastic/apm-lambda-extension/extension"
	"elastic/apm-lambda-extension/logsapi"
	json "encoding/json"
	"github.com/google/uuid"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func initMockServers(eventsChannel chan MockEvent) (*httptest.Server, *httptest.Server, chan struct{}) {

	// Mock APM Server
	hangChan := make(chan struct{})
	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	os.Setenv("ELASTIC_APM_LAMBDA_APM_SERVER", apmServer.URL)
	os.Setenv("ELASTIC_APM_SECRET_TOKEN", "none")

	// Mock Lambda Server
	logsapi.ListenerHost = "localhost"
	lambdaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.RequestURI {
		// Extension registration request
		case "/2020-01-01/extension/register":
			w.Header().Set("Lambda-Extension-Identifier", "b03a29ec-ee63-44cd-8e53-3987a8e8aa8e")
			err := json.NewEncoder(w).Encode(extension.RegisterResponse{
				FunctionName:    "UnitTestingMockLambda",
				FunctionVersion: "$LATEST",
				Handler:         "main_test.mock_lambda",
			})
			if err != nil {
				log.Printf("Could not encode registration response : %v", err)
				return
			}
		case "/2020-01-01/extension/event/next":
			currId := uuid.New().String()
			select {
			case nextEvent := <-eventsChannel:
				sendNextEventInfo(w, currId, nextEvent)
				go processMockEvent(currId, nextEvent, apmServer)
			default:
				finalShutDown := MockEvent{
					Type:              Shutdown,
					ExecutionDuration: 0,
					Timeout:           0,
				}
				sendNextEventInfo(w, currId, finalShutDown)
				go processMockEvent(currId, finalShutDown, apmServer)
			}
		// Logs API subscription request
		case "/2020-08-15/logs":
			w.WriteHeader(http.StatusOK)
		}
	}))

	slicedLambdaURL := strings.Split(lambdaServer.URL, "//")
	strippedLambdaURL := slicedLambdaURL[1]
	os.Setenv("AWS_LAMBDA_RUNTIME_API", strippedLambdaURL)
	extensionClient = extension.NewClient(os.Getenv("AWS_LAMBDA_RUNTIME_API"))

	return lambdaServer, apmServer, hangChan
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
		res, _ := client.Do(req)
		log.Printf("Response seen by the agent : %d", res.StatusCode)
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

func sendNextEventInfo(w http.ResponseWriter, id string, event MockEvent) {
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

	err := json.NewEncoder(w).Encode(nextEventInfo)
	if err != nil {
		log.Printf("Could not encode event : %v", err)
	}
}

func sendLogEvent(requestId string, logEventType logsapi.SubEventType) {
	record := logsapi.LogEventRecord{
		RequestId: requestId,
	}
	logEvent := logsapi.LogEvent{
		Time:   time.Now(),
		Type:   logEventType,
		Record: record,
	}

	// Convert record to JSON (string)
	bufRecord := new(bytes.Buffer)
	err := json.NewEncoder(bufRecord).Encode(record)
	if err != nil {
		log.Printf("Could not encode record : %v", err)
		return
	}
	logEvent.StringRecord = string(bufRecord.Bytes())

	// Convert full log event to JSON
	bufLogEvent := new(bytes.Buffer)
	err = json.NewEncoder(bufLogEvent).Encode([]logsapi.LogEvent{logEvent})
	if err != nil {
		log.Printf("Could not encode record : %v", err)
		return
	}
	host, port, _ := net.SplitHostPort(logsapi.Listener.Addr().String())
	req, _ := http.NewRequest("POST", "http://"+host+":"+port, bufLogEvent)
	client := http.Client{}
	_, err = client.Do(req)
	if err != nil {
		log.Printf("Could not send log event : %v", err)
		return
	}
}

func eventQueueGenerator(inputQueue []MockEvent, eventsChannel chan MockEvent) {
	for _, event := range inputQueue {
		eventsChannel <- event
	}
}

// TESTS

func TestStandardEventsChain(t *testing.T) {
	http.DefaultServeMux = new(http.ServeMux)
	log.Println("Standard Test")

	eventsChannel := make(chan MockEvent, 100)
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
	http.DefaultServeMux = new(http.ServeMux)
	log.Println("APM Server Down")

	eventsChannel := make(chan MockEvent, 100)
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
	http.DefaultServeMux = new(http.ServeMux)
	log.Println("APM Server Hangs")

	eventsChannel := make(chan MockEvent, 100)
	lambdaServer, APMServer, hangChan := initMockServers(eventsChannel)
	defer lambdaServer.Close()
	defer APMServer.Close()

	eventsChain := []MockEvent{
		{Type: InvokeStandard, APMServerBehavior: Hangs, ExecutionDuration: 1, Timeout: 5},
	}
	eventQueueGenerator(eventsChain, eventsChannel)
	start := time.Now()
	main()
	log.Printf("Success : test took %s", time.Since(start))
	hangChan <- struct{}{}
}

func TestAPMServerCrashesDuringExecution(t *testing.T) {
	http.DefaultServeMux = new(http.ServeMux)
	log.Println("APM Server Crashes during execution")

	eventsChannel := make(chan MockEvent, 100)
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
	http.DefaultServeMux = new(http.ServeMux)
	log.Println("AgentData channel is full")

	eventsChannel := make(chan MockEvent, 1000)
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
	http.DefaultServeMux = new(http.ServeMux)
	log.Println("AgentData channel is full, and APM server is slow")
	os.Setenv("ELASTIC_APM_SEND_STRATEGY", "background")
	eventsChannel := make(chan MockEvent, 1000)
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
	http.DefaultServeMux = new(http.ServeMux)
	log.Println("Flush Test")

	eventsChannel := make(chan MockEvent, 100)
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
	http.DefaultServeMux = new(http.ServeMux)
	log.Println("Multiple transactions")

	eventsChannel := make(chan MockEvent, 100)
	lambdaServer, APMServer, _ := initMockServers(eventsChannel)
	defer lambdaServer.Close()
	defer APMServer.Close()

	eventsChain := []MockEvent{
		{Type: InvokeWaitgroupsRace, APMServerBehavior: TimelyResponse, ExecutionDuration: 0.1, Timeout: 500},
	}
	eventQueueGenerator(eventsChain, eventsChannel)
	main()
}
