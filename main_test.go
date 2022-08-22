// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"elastic/apm-lambda-extension/app"
	e2eTesting "elastic/apm-lambda-extension/e2e-testing"
	"elastic/apm-lambda-extension/extension"
	"elastic/apm-lambda-extension/logger"
	"elastic/apm-lambda-extension/logsapi"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type MockEventType string

const (
	InvokeHang                         MockEventType = "Hang"
	InvokeStandard                     MockEventType = "Standard"
	InvokeStandardInfo                 MockEventType = "StandardInfo"
	InvokeStandardFlush                MockEventType = "StandardFlush"
	InvokeStandardMetadata             MockEventType = "StandardMetadata"
	InvokeLateFlush                    MockEventType = "LateFlush"
	InvokeWaitgroupsRace               MockEventType = "InvokeWaitgroupsRace"
	InvokeMultipleTransactionsOverload MockEventType = "MultipleTransactionsOverload"
	Shutdown                           MockEventType = "Shutdown"
)

type MockServerInternals struct {
	Data                string
	WaitForUnlockSignal bool
	UnlockSignalChannel chan struct{}
	WaitGroup           sync.WaitGroup
}

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

type ApmInfo struct {
	BuildDate    time.Time `json:"build_date"`
	BuildSHA     string    `json:"build_sha"`
	PublishReady bool      `json:"publish_ready"`
	Version      string    `json:"version"`
}

func newMockApmServer(t *testing.T, l *zap.SugaredLogger) (*MockServerInternals, *httptest.Server) {
	var apmServerInternals MockServerInternals
	apmServerInternals.WaitForUnlockSignal = true
	apmServerInternals.UnlockSignalChannel = make(chan struct{})
	apmServerMutex := &sync.Mutex{}
	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decompressedBytes, err := e2eTesting.GetDecompressedBytesFromRequest(r)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
		l.Debugf("Event type received by mock APM server : %s", string(decompressedBytes))
		switch APMServerBehavior(decompressedBytes) {
		case TimelyResponse:
			l.Debug("Timely response signal received")
		case SlowResponse:
			l.Debug("Slow response signal received")
			time.Sleep(2 * time.Second)
		case Hangs:
			l.Debug("Hang signal received")
			apmServerMutex.Lock()
			if apmServerInternals.WaitForUnlockSignal {
				<-apmServerInternals.UnlockSignalChannel
				apmServerInternals.WaitForUnlockSignal = false
			}
			apmServerMutex.Unlock()
		case Crashes:
			panic("Server crashed")
		default:
		}
		if r.RequestURI == "/intake/v2/events" {
			w.WriteHeader(http.StatusAccepted)
			apmServerInternals.Data += string(decompressedBytes)
			l.Debug("APM Payload processed")
		} else if r.RequestURI == "/" {
			w.WriteHeader(http.StatusOK)
			infoPayload, err := json.Marshal(ApmInfo{
				BuildDate:    time.Now(),
				BuildSHA:     "7814d524d3602e70b703539c57568cba6964fc20",
				PublishReady: true,
				Version:      "8.1.2",
			})
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
			_, err = w.Write(infoPayload)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}
	}))

	t.Setenv("ELASTIC_APM_LAMBDA_APM_SERVER", apmServer.URL)
	t.Setenv("ELASTIC_APM_SECRET_TOKEN", "none")

	t.Cleanup(func() { apmServer.Close() })
	return &apmServerInternals, apmServer
}

func newMockLambdaServer(t *testing.T, logsapiAddr string, eventsChannel chan MockEvent, l *zap.SugaredLogger) *MockServerInternals {
	var lambdaServerInternals MockServerInternals
	lambdaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.RequestURI {
		// Extension registration request
		case "/2020-01-01/extension/register":
			w.Header().Set("Lambda-Extension-Identifier", "b03a29ec-ee63-44cd-8e53-3987a8e8aa8e")
			if err := json.NewEncoder(w).Encode(extension.RegisterResponse{
				FunctionName:    "UnitTestingMockLambda",
				FunctionVersion: "$LATEST",
				Handler:         "main_test.mock_lambda",
			}); err != nil {
				l.Fatalf("Could not encode registration response : %v", err)
				return
			}
		case "/2020-01-01/extension/event/next":
			lambdaServerInternals.WaitGroup.Wait()
			currId := uuid.New().String()
			select {
			case nextEvent := <-eventsChannel:
				sendNextEventInfo(w, currId, nextEvent, l)
				go processMockEvent(currId, nextEvent, os.Getenv("ELASTIC_APM_DATA_RECEIVER_SERVER_PORT"), logsapiAddr, &lambdaServerInternals, l)
			default:
				finalShutDown := MockEvent{
					Type:              Shutdown,
					ExecutionDuration: 0,
					Timeout:           0,
				}
				sendNextEventInfo(w, currId, finalShutDown, l)
				go processMockEvent(currId, finalShutDown, os.Getenv("ELASTIC_APM_DATA_RECEIVER_SERVER_PORT"), logsapiAddr, &lambdaServerInternals, l)
			}
		// Logs API subscription request
		case "/2020-08-15/logs":
			w.WriteHeader(http.StatusOK)
		}
	}))

	slicedLambdaURL := strings.Split(lambdaServer.URL, "//")
	strippedLambdaURL := slicedLambdaURL[1]
	t.Setenv("AWS_LAMBDA_RUNTIME_API", strippedLambdaURL)

	// Find unused port for the extension to listen to
	extensionPort, err := e2eTesting.GetFreePort()
	if err != nil {
		l.Errorf("Could not find free port for the extension to listen on : %v", err)
		extensionPort = 8200
	}
	t.Setenv("ELASTIC_APM_DATA_RECEIVER_SERVER_PORT", fmt.Sprint(extensionPort))

	t.Cleanup(func() { lambdaServer.Close() })
	return &lambdaServerInternals
}

func newTestStructs(t *testing.T) chan MockEvent {
	http.DefaultServeMux = new(http.ServeMux)
	_, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel() })
	eventsChannel := make(chan MockEvent, 100)
	return eventsChannel
}

func processMockEvent(currId string, event MockEvent, extensionPort string, logsapiAddr string, internals *MockServerInternals, l *zap.SugaredLogger) {
	sendLogEvent(logsapiAddr, currId, logsapi.Start, l)
	client := http.Client{}

	// Use a custom transport with a low timeout
	tr := http.DefaultTransport.(*http.Transport)
	tr.ResponseHeaderTimeout = 200 * time.Millisecond
	client.Transport = tr

	sendRuntimeDone := true
	sendMetrics := true

	// Used in LateFlush events to make sure that
	// the request is sent after the RuntimeDone.
	ch := make(chan struct{})
	defer close(ch)

	// float values were silently ignored (casted to int)
	// Multiply before casting to support more values.
	delay := time.Duration(event.ExecutionDuration * float64(time.Second))

	switch event.Type {
	case InvokeHang:
		time.Sleep(time.Duration(event.Timeout * float64(time.Second)))
	case InvokeStandard:
		time.Sleep(delay)
		req, _ := http.NewRequest("POST", fmt.Sprintf("http://localhost:%s/intake/v2/events", extensionPort), bytes.NewBuffer([]byte(event.APMServerBehavior)))
		res, _ := client.Do(req)
		l.Debugf("Response seen by the agent : %d", res.StatusCode)
	case InvokeStandardMetadata:
		time.Sleep(delay)
		metadata := `{"metadata":{"service":{"name":"1234_service-12a3","version":"5.1.3","environment":"staging","agent":{"name":"elastic-node","version":"3.14.0"},"framework":{"name":"Express","version":"1.2.3"},"language":{"name":"ecmascript","version":"8"},"runtime":{"name":"node","version":"8.0.0"},"node":{"configured_name":"node-123"}},"user":{"username":"bar","id":"123user","email":"bar@user.com"},"labels":{"tag0":null,"tag1":"one","tag2":2},"process":{"pid":1234,"ppid":6789,"title":"node","argv":["node","server.js"]},"system":{"architecture":"x64","hostname":"prod1.example.com","platform":"darwin","container":{"id":"container-id"},"kubernetes":{"namespace":"namespace1","node":{"name":"node-name"},"pod":{"name":"pod-name","uid":"pod-uid"}}},"cloud":{"provider":"cloud_provider","region":"cloud_region","availability_zone":"cloud_availability_zone","instance":{"id":"instance_id","name":"instance_name"},"machine":{"type":"machine_type"},"account":{"id":"account_id","name":"account_name"},"project":{"id":"project_id","name":"project_name"},"service":{"name":"lambda"}}}}`
		req, _ := http.NewRequest("POST", fmt.Sprintf("http://localhost:%s/intake/v2/events", extensionPort), bytes.NewBuffer([]byte(metadata)))
		res, _ := client.Do(req)
		l.Debugf("Response seen by the agent : %d", res.StatusCode)
	case InvokeStandardFlush:
		time.Sleep(delay)
		reqData, _ := http.NewRequest("POST", fmt.Sprintf("http://localhost:%s/intake/v2/events?flushed=true", extensionPort), bytes.NewBuffer([]byte(event.APMServerBehavior)))
		if _, err := client.Do(reqData); err != nil {
			l.Error(err.Error())
		}
	case InvokeLateFlush:
		time.Sleep(delay)
		reqData, _ := http.NewRequest("POST", fmt.Sprintf("http://localhost:%s/intake/v2/events?flushed=true", extensionPort), bytes.NewBuffer([]byte(event.APMServerBehavior)))
		internals.WaitGroup.Add(1)
		go func() {
			<-ch
			if _, err := client.Do(reqData); err != nil {
				l.Error(err.Error())
			}
			internals.WaitGroup.Done()
		}()
		// For this specific scenario, we do not want to see metrics in the APM Server logs (in order to easily check if the logs contain to "TimelyResponse" back to back).
		sendMetrics = false
	case InvokeWaitgroupsRace:
		time.Sleep(delay)
		reqData0, _ := http.NewRequest("POST", fmt.Sprintf("http://localhost:%s/intake/v2/events", extensionPort), bytes.NewBuffer([]byte(event.APMServerBehavior)))
		reqData1, _ := http.NewRequest("POST", fmt.Sprintf("http://localhost:%s/intake/v2/events", extensionPort), bytes.NewBuffer([]byte(event.APMServerBehavior)))
		if _, err := client.Do(reqData0); err != nil {
			l.Error(err.Error())
		}
		if _, err := client.Do(reqData1); err != nil {
			l.Error(err.Error())
		}
		time.Sleep(650 * time.Microsecond)
	case InvokeMultipleTransactionsOverload:
		wg := sync.WaitGroup{}
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				time.Sleep(delay)
				reqData, _ := http.NewRequest("POST", fmt.Sprintf("http://localhost:%s/intake/v2/events", extensionPort), bytes.NewBuffer([]byte(event.APMServerBehavior)))
				if _, err := client.Do(reqData); err != nil {
					l.Error(err.Error())
				}
				wg.Done()
			}()
		}
		wg.Wait()
	case InvokeStandardInfo:
		time.Sleep(delay)
		req, _ := http.NewRequest("POST", fmt.Sprintf("http://localhost:%s/", extensionPort), bytes.NewBuffer([]byte(event.APMServerBehavior)))
		res, err := client.Do(req)
		if err != nil {
			l.Errorf("No response following info request : %v", err)
			break
		}
		var rawBytes []byte
		if res.Body != nil {
			rawBytes, _ = io.ReadAll(res.Body)
		}
		internals.Data += string(rawBytes)
		l.Debugf("Response seen by the agent : %d", res.StatusCode)
	case Shutdown:
	}
	if sendRuntimeDone {
		sendLogEvent(logsapiAddr, currId, logsapi.RuntimeDone, l)
	}
	if sendMetrics {
		sendLogEvent(logsapiAddr, currId, logsapi.Report, l)
	}
}

func sendNextEventInfo(w http.ResponseWriter, id string, event MockEvent, l *zap.SugaredLogger) {
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

	if err := json.NewEncoder(w).Encode(nextEventInfo); err != nil {
		l.Errorf("Could not encode event : %v", err)
	}
}

func sendLogEvent(logsapiAddr string, requestId string, logEventType logsapi.SubEventType, l *zap.SugaredLogger) {
	record := logsapi.LogEventRecord{
		RequestID: requestId,
	}
	if logEventType == logsapi.Report {
		record.Metrics = logsapi.PlatformMetrics{
			BilledDurationMs: 60,
			DurationMs:       59.9,
			MemorySizeMB:     128,
			MaxMemoryUsedMB:  60,
			InitDurationMs:   500,
		}
	}

	logEvent := logsapi.LogEvent{
		Time:   time.Now(),
		Type:   logEventType,
		Record: record,
	}

	// Convert record to JSON (string)
	bufRecord := new(bytes.Buffer)
	if err := json.NewEncoder(bufRecord).Encode(record); err != nil {
		l.Errorf("Could not encode record : %v", err)
		return
	}
	logEvent.StringRecord = bufRecord.String()

	// Convert full log event to JSON
	bufLogEvent := new(bytes.Buffer)
	if err := json.NewEncoder(bufLogEvent).Encode([]logsapi.LogEvent{logEvent}); err != nil {
		l.Errorf("Could not encode record : %v", err)
		return
	}

	req, err := http.NewRequest(http.MethodPost, "http://"+logsapiAddr, bufLogEvent)
	if err != nil {
		l.Errorf("Could not create logs api request: %v", err)
		return
	}

	client := http.Client{}
	if _, err := client.Do(req); err != nil {
		l.Errorf("Could not send log event : %v", err)
		return
	}
}

func eventQueueGenerator(inputQueue []MockEvent, eventsChannel chan MockEvent) {
	for _, event := range inputQueue {
		eventsChannel <- event
	}
}

// TestStandardEventsChain checks a nominal sequence of events (fast APM server, only one standard event)
func TestStandardEventsChain(t *testing.T) {
	l, err := logger.New(logger.WithLevel(zapcore.DebugLevel))
	require.NoError(t, err)

	eventsChannel := newTestStructs(t)
	apmServerInternals, _ := newMockApmServer(t, l)
	logsapiAddr := randomAddr()
	newMockLambdaServer(t, logsapiAddr, eventsChannel, l)

	eventsChain := []MockEvent{
		{Type: InvokeStandard, APMServerBehavior: TimelyResponse, ExecutionDuration: 1, Timeout: 5},
	}
	eventQueueGenerator(eventsChain, eventsChannel)
	select {
	case <-runApp(t, logsapiAddr):
		assert.Contains(t, apmServerInternals.Data, TimelyResponse)
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for app to finish")
	}
}

// TestFlush checks if the flushed param does not cause a panic or an unexpected behavior
func TestFlush(t *testing.T) {
	l, err := logger.New(logger.WithLevel(zapcore.DebugLevel))
	require.NoError(t, err)

	eventsChannel := newTestStructs(t)
	apmServerInternals, _ := newMockApmServer(t, l)
	logsapiAddr := randomAddr()
	newMockLambdaServer(t, logsapiAddr, eventsChannel, l)

	eventsChain := []MockEvent{
		{Type: InvokeStandardFlush, APMServerBehavior: TimelyResponse, ExecutionDuration: 1, Timeout: 5},
	}
	eventQueueGenerator(eventsChain, eventsChannel)
	select {
	case <-runApp(t, logsapiAddr):
		assert.Contains(t, apmServerInternals.Data, TimelyResponse)
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for app to finish")
	}
}

// TestLateFlush checks if there is no race condition between RuntimeDone and AgentDone
// The test is built so that the AgentDone signal is received after RuntimeDone, which causes the next event to be interrupted.
func TestLateFlush(t *testing.T) {
	l, err := logger.New(logger.WithLevel(zapcore.DebugLevel))
	require.NoError(t, err)

	eventsChannel := newTestStructs(t)
	apmServerInternals, _ := newMockApmServer(t, l)
	logsapiAddr := randomAddr()
	newMockLambdaServer(t, logsapiAddr, eventsChannel, l)

	eventsChain := []MockEvent{
		{Type: InvokeLateFlush, APMServerBehavior: TimelyResponse, ExecutionDuration: 0, Timeout: 5},
		{Type: InvokeStandard, APMServerBehavior: TimelyResponse, ExecutionDuration: 0, Timeout: 5},
	}
	eventQueueGenerator(eventsChain, eventsChannel)
	select {
	case <-runApp(t, logsapiAddr):
		assert.Contains(t, apmServerInternals.Data, TimelyResponse+TimelyResponse)
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for app to finish")
	}
}

// TestWaitGroup checks if there is no race condition between the main waitgroups (issue #128)
func TestWaitGroup(t *testing.T) {
	l, err := logger.New(logger.WithLevel(zapcore.DebugLevel))
	require.NoError(t, err)

	eventsChannel := newTestStructs(t)
	apmServerInternals, _ := newMockApmServer(t, l)
	logsapiAddr := randomAddr()
	newMockLambdaServer(t, logsapiAddr, eventsChannel, l)

	eventsChain := []MockEvent{
		{Type: InvokeWaitgroupsRace, APMServerBehavior: TimelyResponse, ExecutionDuration: 1, Timeout: 500},
	}
	eventQueueGenerator(eventsChain, eventsChannel)
	select {
	case <-runApp(t, logsapiAddr):
		assert.Contains(t, apmServerInternals.Data, TimelyResponse)
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for app to finish")
	}
}

// TestAPMServerDown tests that main does not panic nor runs indefinitely when the APM server is inactive.
func TestAPMServerDown(t *testing.T) {
	l, err := logger.New(logger.WithLevel(zapcore.DebugLevel))
	require.NoError(t, err)

	eventsChannel := newTestStructs(t)
	apmServerInternals, apmServer := newMockApmServer(t, l)
	logsapiAddr := randomAddr()
	newMockLambdaServer(t, logsapiAddr, eventsChannel, l)

	apmServer.Close()
	eventsChain := []MockEvent{
		{Type: InvokeStandard, APMServerBehavior: TimelyResponse, ExecutionDuration: 1, Timeout: 5},
	}
	eventQueueGenerator(eventsChain, eventsChannel)
	select {
	case <-runApp(t, logsapiAddr):
		assert.NotContains(t, apmServerInternals.Data, TimelyResponse)
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for app to finish")
	}
}

// TestAPMServerHangs tests that main does not panic nor runs indefinitely when the APM server does not respond.
func TestAPMServerHangs(t *testing.T) {
	l, err := logger.New(logger.WithLevel(zapcore.DebugLevel))
	require.NoError(t, err)

	eventsChannel := newTestStructs(t)
	apmServerInternals, _ := newMockApmServer(t, l)
	logsapiAddr := randomAddr()
	newMockLambdaServer(t, logsapiAddr, eventsChannel, l)

	eventsChain := []MockEvent{
		{Type: InvokeStandard, APMServerBehavior: Hangs, ExecutionDuration: 1, Timeout: 500},
	}
	eventQueueGenerator(eventsChain, eventsChannel)
	select {
	case <-runApp(t, logsapiAddr):
		assert.NotContains(t, apmServerInternals.Data, Hangs)
		apmServerInternals.UnlockSignalChannel <- struct{}{}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for app to finish")
	}
}

// TestAPMServerRecovery tests a scenario where the APM server recovers after hanging.
// The default forwarder timeout is 3 seconds, so we wait 5 seconds until we unlock that hanging state.
// Hence, the APM server is waked up just in time to process the TimelyResponse data frame.
func TestAPMServerRecovery(t *testing.T) {
	l, err := logger.New(logger.WithLevel(zapcore.DebugLevel))
	require.NoError(t, err)

	eventsChannel := newTestStructs(t)
	apmServerInternals, _ := newMockApmServer(t, l)
	logsapiAddr := randomAddr()
	newMockLambdaServer(t, logsapiAddr, eventsChannel, l)

	t.Setenv("ELASTIC_APM_DATA_FORWARDER_TIMEOUT_SECONDS", "1")

	eventsChain := []MockEvent{
		{Type: InvokeStandard, APMServerBehavior: Hangs, ExecutionDuration: 1, Timeout: 5},
		{Type: InvokeStandard, APMServerBehavior: TimelyResponse, ExecutionDuration: 1, Timeout: 5},
	}
	eventQueueGenerator(eventsChain, eventsChannel)
	go func() {
		time.Sleep(2500 * time.Millisecond) // Cannot multiply time.Second by a float
		apmServerInternals.UnlockSignalChannel <- struct{}{}
	}()
	select {
	case <-runApp(t, logsapiAddr):
		assert.Contains(t, apmServerInternals.Data, Hangs)
		assert.Contains(t, apmServerInternals.Data, TimelyResponse)
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for app to finish")
	}

}

// TestGracePeriodHangs verifies that the WaitforGracePeriod goroutine ends when main() ends.
// This can be checked by asserting that apmTransportStatus is pending after the execution of main.
func TestGracePeriodHangs(t *testing.T) {
	l, err := logger.New(logger.WithLevel(zapcore.DebugLevel))
	require.NoError(t, err)

	eventsChannel := newTestStructs(t)
	apmServerInternals, _ := newMockApmServer(t, l)
	logsapiAddr := randomAddr()
	newMockLambdaServer(t, logsapiAddr, eventsChannel, l)

	eventsChain := []MockEvent{
		{Type: InvokeStandard, APMServerBehavior: Hangs, ExecutionDuration: 1, Timeout: 500},
	}
	eventQueueGenerator(eventsChain, eventsChannel)
	select {
	case <-runApp(t, logsapiAddr):
		time.Sleep(100 * time.Millisecond)
		apmServerInternals.UnlockSignalChannel <- struct{}{}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for app to finish")
	}

}

// TestAPMServerCrashesDuringExecution tests that main does not panic nor runs indefinitely when the APM server crashes
// during execution.
func TestAPMServerCrashesDuringExecution(t *testing.T) {
	l, err := logger.New(logger.WithLevel(zapcore.DebugLevel))
	require.NoError(t, err)

	eventsChannel := newTestStructs(t)
	apmServerInternals, _ := newMockApmServer(t, l)
	logsapiAddr := randomAddr()
	newMockLambdaServer(t, logsapiAddr, eventsChannel, l)

	eventsChain := []MockEvent{
		{Type: InvokeStandard, APMServerBehavior: Crashes, ExecutionDuration: 1, Timeout: 5},
	}
	eventQueueGenerator(eventsChain, eventsChannel)
	select {
	case <-runApp(t, logsapiAddr):
		assert.NotContains(t, apmServerInternals.Data, Crashes)
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for app to finish")
	}
}

// TestFullChannel checks that an overload of APM data chunks is handled correctly, events dropped beyond the 100th one
// if no space left in channel, no panic, no infinite hanging.
func TestFullChannel(t *testing.T) {
	l, err := logger.New(logger.WithLevel(zapcore.DebugLevel))
	require.NoError(t, err)

	eventsChannel := newTestStructs(t)
	apmServerInternals, _ := newMockApmServer(t, l)
	logsapiAddr := randomAddr()
	newMockLambdaServer(t, logsapiAddr, eventsChannel, l)

	// Use a smaller buffer size to make it easier to reproduce
	t.Setenv("ELASTIC_APM_LAMBDA_AGENT_DATA_BUFFER_SIZE", "1")

	eventsChain := []MockEvent{
		{Type: InvokeMultipleTransactionsOverload, APMServerBehavior: TimelyResponse, ExecutionDuration: 0.1, Timeout: 5},
	}
	eventQueueGenerator(eventsChain, eventsChannel)
	select {
	case <-runApp(t, logsapiAddr):
		assert.Contains(t, apmServerInternals.Data, TimelyResponse)
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for app to finish")
	}
}

// TestFullChannelSlowAPMServer tests what happens when the APM Data channel is full and the APM server is slow
// (send strategy : background)
func TestFullChannelSlowAPMServer(t *testing.T) {
	l, err := logger.New(logger.WithLevel(zapcore.DebugLevel))
	require.NoError(t, err)

	eventsChannel := newTestStructs(t)
	newMockApmServer(t, l)
	logsapiAddr := randomAddr()
	newMockLambdaServer(t, logsapiAddr, eventsChannel, l)

	t.Setenv("ELASTIC_APM_SEND_STRATEGY", "background")
	// Use a smaller buffer size to make it easier to reproduce
	t.Setenv("ELASTIC_APM_LAMBDA_AGENT_DATA_BUFFER_SIZE", "1")

	eventsChain := []MockEvent{
		{Type: InvokeMultipleTransactionsOverload, APMServerBehavior: SlowResponse, ExecutionDuration: 0.01, Timeout: 5},
	}
	eventQueueGenerator(eventsChain, eventsChannel)
	select {
	case <-runApp(t, logsapiAddr):
		// The test should not hang
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for app to finish")
	}
}

// TestInfoRequest checks if the extension is able to retrieve APM server info (/ endpoint) (fast APM server, only one standard event)
func TestInfoRequest(t *testing.T) {
	l, err := logger.New(logger.WithLevel(zapcore.DebugLevel))
	require.NoError(t, err)

	eventsChannel := newTestStructs(t)
	newMockApmServer(t, l)
	logsapiAddr := randomAddr()
	lambdaServerInternals := newMockLambdaServer(t, logsapiAddr, eventsChannel, l)

	eventsChain := []MockEvent{
		{Type: InvokeStandardInfo, APMServerBehavior: TimelyResponse, ExecutionDuration: 1, Timeout: 5},
	}
	eventQueueGenerator(eventsChain, eventsChannel)
	select {
	case <-runApp(t, logsapiAddr):
		assert.Contains(t, lambdaServerInternals.Data, "7814d524d3602e70b703539c57568cba6964fc20")
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for app to finish")
	}
}

// TestInfoRequest checks if the extension times out when unable to retrieve APM server info (/ endpoint)
func TestInfoRequestHangs(t *testing.T) {
	l, err := logger.New(logger.WithLevel(zapcore.DebugLevel))
	require.NoError(t, err)

	eventsChannel := newTestStructs(t)
	apmServerInternals, _ := newMockApmServer(t, l)
	logsapiAddr := randomAddr()
	lambdaServerInternals := newMockLambdaServer(t, logsapiAddr, eventsChannel, l)

	eventsChain := []MockEvent{
		{Type: InvokeStandardInfo, APMServerBehavior: Hangs, ExecutionDuration: 1, Timeout: 500},
	}
	eventQueueGenerator(eventsChain, eventsChannel)
	select {
	case <-runApp(t, logsapiAddr):
		time.Sleep(2 * time.Second)
		assert.NotContains(t, lambdaServerInternals.Data, "7814d524d3602e70b703539c57568cba6964fc20")
		apmServerInternals.UnlockSignalChannel <- struct{}{}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for app to finish")
	}
}

// TestMetricsWithoutMetadata checks if the extension sends metrics corresponding to invocation n during invocation
// n+1, even if the metadata container was not populated
func TestMetricsWithoutMetadata(t *testing.T) {
	l, err := logger.New(logger.WithLevel(zapcore.DebugLevel))
	require.NoError(t, err)

	eventsChannel := newTestStructs(t)
	apmServerInternals, _ := newMockApmServer(t, l)
	logsapiAddr := randomAddr()
	newMockLambdaServer(t, logsapiAddr, eventsChannel, l)

	eventsChain := []MockEvent{
		{Type: InvokeStandard, APMServerBehavior: TimelyResponse, ExecutionDuration: 1, Timeout: 5},
		{Type: InvokeStandard, APMServerBehavior: TimelyResponse, ExecutionDuration: 1, Timeout: 5},
	}
	eventQueueGenerator(eventsChain, eventsChannel)

	select {
	case <-runApp(t, logsapiAddr):
		assert.Contains(t, apmServerInternals.Data, `faas.billed_duration":{"value":60`)
		assert.Contains(t, apmServerInternals.Data, `faas.duration":{"value":59.9`)
		assert.Contains(t, apmServerInternals.Data, `faas.coldstart_duration":{"value":500`)
		assert.Contains(t, apmServerInternals.Data, `faas.timeout":{"value":5000}`)
		assert.Contains(t, apmServerInternals.Data, `system.memory.actual.free":{"value":7.1303168e+07`)
		assert.Contains(t, apmServerInternals.Data, `system.memory.total":{"value":1.34217728e+08`)
		assert.Contains(t, apmServerInternals.Data, `coldstart":true`)
		assert.Contains(t, apmServerInternals.Data, `execution":`)
		assert.Contains(t, apmServerInternals.Data, `id":"arn:aws:lambda:eu-central-1:627286350134:function:main_unit_test"`)
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for app to finish")
	}
}

// TestMetricsWithMetadata checks if the extension sends metrics corresponding to invocation n during invocation
// n+1, even if the metadata container was not populated
func TestMetricsWithMetadata(t *testing.T) {
	l, err := logger.New(logger.WithLevel(zapcore.DebugLevel))
	require.NoError(t, err)

	eventsChannel := newTestStructs(t)
	apmServerInternals, _ := newMockApmServer(t, l)
	logsapiAddr := randomAddr()
	newMockLambdaServer(t, logsapiAddr, eventsChannel, l)

	eventsChain := []MockEvent{
		{Type: InvokeStandardMetadata, APMServerBehavior: TimelyResponse, ExecutionDuration: 1, Timeout: 5},
		{Type: InvokeStandardMetadata, APMServerBehavior: TimelyResponse, ExecutionDuration: 1, Timeout: 5},
	}
	eventQueueGenerator(eventsChain, eventsChannel)

	select {
	case <-runApp(t, logsapiAddr):
		assert.Contains(t, apmServerInternals.Data, `{"metadata":{"service":{"name":"1234_service-12a3","version":"5.1.3","environment":"staging","agent":{"name":"elastic-node","version":"3.14.0"},"framework":{"name":"Express","version":"1.2.3"},"language":{"name":"ecmascript","version":"8"},"runtime":{"name":"node","version":"8.0.0"},"node":{"configured_name":"node-123"}},"user":{"username":"bar","id":"123user","email":"bar@user.com"},"labels":{"tag0":null,"tag1":"one","tag2":2},"process":{"pid":1234,"ppid":6789,"title":"node","argv":["node","server.js"]},"system":{"architecture":"x64","hostname":"prod1.example.com","platform":"darwin","container":{"id":"container-id"},"kubernetes":{"namespace":"namespace1","node":{"name":"node-name"},"pod":{"name":"pod-name","uid":"pod-uid"}}},"cloud":{"provider":"cloud_provider","region":"cloud_region","availability_zone":"cloud_availability_zone","instance":{"id":"instance_id","name":"instance_name"},"machine":{"type":"machine_type"},"account":{"id":"account_id","name":"account_name"},"project":{"id":"project_id","name":"project_name"},"service":{"name":"lambda"}}}}`)
		assert.Contains(t, apmServerInternals.Data, `faas.billed_duration":{"value":60`)
		assert.Contains(t, apmServerInternals.Data, `faas.duration":{"value":59.9`)
		assert.Contains(t, apmServerInternals.Data, `faas.coldstart_duration":{"value":500`)
		assert.Contains(t, apmServerInternals.Data, `faas.timeout":{"value":5000}`)
		assert.Contains(t, apmServerInternals.Data, `coldstart":true`)
		assert.Contains(t, apmServerInternals.Data, `execution"`)
		assert.Contains(t, apmServerInternals.Data, `id":"arn:aws:lambda:eu-central-1:627286350134:function:main_unit_test"`)
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for app to finish")
	}
}

func runApp(t *testing.T, logsapiAddr string) <-chan struct{} {
	ctx, cancel := context.WithCancel(context.Background())

	app, err := app.New(
		app.WithExtensionName("apm-lambda-extension"),
		app.WithLambdaRuntimeAPI(os.Getenv("AWS_LAMBDA_RUNTIME_API")),
		app.WithLogLevel("debug"),
		app.WithLogsapiAddress(logsapiAddr),
	)
	require.NoError(t, err)

	go func() {
		require.NoError(t, app.Run(ctx))
		cancel()
	}()

	return ctx.Done()
}

func randomAddr() string {
	// we cannot return a port that is already in use or it
	// would return an error when creating the server.
	// The solution is to spawn a test server to get a random
	// port and immediately close it so that we can use the port.
	s := httptest.NewServer(nil)
	addr := s.Listener.Addr().String()
	s.Close()

	return addr
}
