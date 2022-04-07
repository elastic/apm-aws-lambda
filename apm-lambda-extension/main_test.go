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
	e2eTesting "elastic/apm-lambda-extension/e2e-testing"
	"elastic/apm-lambda-extension/extension"
	"elastic/apm-lambda-extension/logsapi"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/suite"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func initMockServers(eventsChannel chan MockEvent) (*httptest.Server, *httptest.Server, *APMServerInternals) {

	// Mock APM Server
	var apmServerInternals APMServerInternals
	apmServerInternals.WaitForUnlockSignal = true
	apmServerInternals.UnlockSignalChannel = make(chan struct{})
	apmServerMutex := &sync.Mutex{}
	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/intake/v2/events" {
			decompressedBytes, err := e2eTesting.GetDecompressedBytesFromRequest(r)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
			extension.Log.Debugf("Event type received by mock APM server : %s", string(decompressedBytes))
			switch APMServerBehavior(decompressedBytes) {
			case TimelyResponse:
				extension.Log.Debug("Timely response received")
				apmServerInternals.Data += string(decompressedBytes)
				w.WriteHeader(http.StatusAccepted)
				extension.Log.Debug("Timely response processed")
			case SlowResponse:
				apmServerInternals.Data += string(decompressedBytes)
				time.Sleep(2 * time.Second)
				w.WriteHeader(http.StatusAccepted)
			case Hangs:
				extension.Log.Debug("Hang signal received")
				apmServerMutex.Lock()
				if apmServerInternals.WaitForUnlockSignal {
					<-apmServerInternals.UnlockSignalChannel
					apmServerInternals.WaitForUnlockSignal = false
				}
				apmServerInternals.Data += string(decompressedBytes)
				apmServerMutex.Unlock()
				extension.Log.Debug("Hang signal processed")
			case Crashes:
				panic("Server crashed")
			default:
				w.WriteHeader(http.StatusInternalServerError)
			}
		}
	}))
	if err := os.Setenv("ELASTIC_APM_LAMBDA_APM_SERVER", apmServer.URL); err != nil {
		extension.Log.Criticalf("Could not set environment variable : %v", err)
		return nil, nil, nil
	}
	if err := os.Setenv("ELASTIC_APM_SECRET_TOKEN", "none"); err != nil {
		extension.Log.Criticalf("Could not set environment variable : %v", err)
		return nil, nil, nil
	}

	// Mock Lambda Server
	logsapi.ListenerHost = "localhost"
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
				extension.Log.Criticalf("Could not encode registration response : %v", err)
				return
			}
		case "/2020-01-01/extension/event/next":
			currId := uuid.New().String()
			select {
			case nextEvent := <-eventsChannel:
				sendNextEventInfo(w, currId, nextEvent)
				go processMockEvent(currId, nextEvent, os.Getenv("ELASTIC_APM_DATA_RECEIVER_SERVER_PORT"))
			default:
				finalShutDown := MockEvent{
					Type:              Shutdown,
					ExecutionDuration: 0,
					Timeout:           0,
				}
				sendNextEventInfo(w, currId, finalShutDown)
				go processMockEvent(currId, finalShutDown, os.Getenv("ELASTIC_APM_DATA_RECEIVER_SERVER_PORT"))
			}
		// Logs API subscription request
		case "/2020-08-15/logs":
			w.WriteHeader(http.StatusOK)
		}
	}))

	slicedLambdaURL := strings.Split(lambdaServer.URL, "//")
	strippedLambdaURL := slicedLambdaURL[1]
	if err := os.Setenv("AWS_LAMBDA_RUNTIME_API", strippedLambdaURL); err != nil {
		extension.Log.Criticalf("Could not set environment variable : %v", err)
		return nil, nil, nil
	}
	extensionClient = extension.NewClient(os.Getenv("AWS_LAMBDA_RUNTIME_API"))

	// Find unused port for the extension to listen to
	extensionPort, err := e2eTesting.GetFreePort()
	if err != nil {
		extension.Log.Errorf("Could not find free port for the extension to listen on : %v", err)
		extensionPort = 8200
	}
	if err = os.Setenv("ELASTIC_APM_DATA_RECEIVER_SERVER_PORT", fmt.Sprint(extensionPort)); err != nil {
		extension.Log.Criticalf("Could not set environment variable : %v", err)
		return nil, nil, nil
	}

	return lambdaServer, apmServer, &apmServerInternals
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

type APMServerInternals struct {
	Data                string
	WaitForUnlockSignal bool
	UnlockSignalChannel chan struct{}
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

func processMockEvent(currId string, event MockEvent, extensionPort string) {
	sendLogEvent(currId, "platform.start")
	client := http.Client{}
	switch event.Type {
	case InvokeHang:
		time.Sleep(time.Duration(event.Timeout) * time.Second)
	case InvokeStandard:
		time.Sleep(time.Duration(event.ExecutionDuration) * time.Second)
		req, _ := http.NewRequest("POST", fmt.Sprintf("http://localhost:%s/intake/v2/events", extensionPort), bytes.NewBuffer([]byte(event.APMServerBehavior)))
		res, _ := client.Do(req)
		extension.Log.Tracef("Response seen by the agent : %d", res.StatusCode)
	case InvokeStandardFlush:
		time.Sleep(time.Duration(event.ExecutionDuration) * time.Second)
		reqData, _ := http.NewRequest("POST", fmt.Sprintf("http://localhost:%s/intake/v2/events?flushed=true", extensionPort), bytes.NewBuffer([]byte(event.APMServerBehavior)))
		if _, err := client.Do(reqData); err != nil {
			extension.Log.Error(err.Error())
		}
	case InvokeWaitgroupsRace:
		time.Sleep(time.Duration(event.ExecutionDuration) * time.Second)
		reqData0, _ := http.NewRequest("POST", fmt.Sprintf("http://localhost:%s/intake/v2/events", extensionPort), bytes.NewBuffer([]byte(event.APMServerBehavior)))
		reqData1, _ := http.NewRequest("POST", fmt.Sprintf("http://localhost:%s/intake/v2/events", extensionPort), bytes.NewBuffer([]byte(event.APMServerBehavior)))
		if _, err := client.Do(reqData0); err != nil {
			extension.Log.Error(err.Error())
		}
		if _, err := client.Do(reqData1); err != nil {
			extension.Log.Error(err.Error())
		}
		time.Sleep(650 * time.Microsecond)
	case InvokeMultipleTransactionsOverload:
		wg := sync.WaitGroup{}
		for i := 0; i < 200; i++ {
			wg.Add(1)
			go func() {
				time.Sleep(time.Duration(event.ExecutionDuration) * time.Second)
				reqData, _ := http.NewRequest("POST", fmt.Sprintf("http://localhost:%s/intake/v2/events", extensionPort), bytes.NewBuffer([]byte(event.APMServerBehavior)))
				if _, err := client.Do(reqData); err != nil {
					extension.Log.Error(err.Error())
				}
				wg.Done()
			}()
		}
		wg.Wait()
	case Shutdown:
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

	if err := json.NewEncoder(w).Encode(nextEventInfo); err != nil {
		extension.Log.Errorf("Could not encode event : %v", err)
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
	if err := json.NewEncoder(bufRecord).Encode(record); err != nil {
		extension.Log.Errorf("Could not encode record : %v", err)
		return
	}
	logEvent.StringRecord = bufRecord.String()

	// Convert full log event to JSON
	bufLogEvent := new(bytes.Buffer)
	if err := json.NewEncoder(bufLogEvent).Encode([]logsapi.LogEvent{logEvent}); err != nil {
		extension.Log.Errorf("Could not encode record : %v", err)
		return
	}
	host, port, _ := net.SplitHostPort(logsapi.Listener.Addr().String())
	req, _ := http.NewRequest("POST", "http://"+host+":"+port, bufLogEvent)
	client := http.Client{}
	if _, err := client.Do(req); err != nil {
		extension.Log.Errorf("Could not send log event : %v", err)
		return
	}
}

func eventQueueGenerator(inputQueue []MockEvent, eventsChannel chan MockEvent) {
	for _, event := range inputQueue {
		eventsChannel <- event
	}
}

// TESTS
type MainUnitTestsSuite struct {
	suite.Suite
	eventsChannel      chan MockEvent
	lambdaServer       *httptest.Server
	apmServer          *httptest.Server
	apmServerInternals *APMServerInternals
	ctx                context.Context
	cancel             context.CancelFunc
}

func TestMainUnitTestsSuite(t *testing.T) {
	suite.Run(t, new(MainUnitTestsSuite))
}

// This function executes before each test case
func (suite *MainUnitTestsSuite) SetupTest() {
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
	http.DefaultServeMux = new(http.ServeMux)
	suite.eventsChannel = make(chan MockEvent, 100)
	suite.lambdaServer, suite.apmServer, suite.apmServerInternals = initMockServers(suite.eventsChannel)
	extension.SetApmServerTransportState(extension.Healthy, suite.ctx)
}

// This function executes after each test case
func (suite *MainUnitTestsSuite) TearDownTest() {
	suite.lambdaServer.Close()
	suite.apmServer.Close()
	suite.cancel()
}

// TestStandardEventsChain checks a nominal sequence of events (fast APM server, only one standard event)
func (suite *MainUnitTestsSuite) TestStandardEventsChain() {
	eventsChain := []MockEvent{
		{Type: InvokeStandard, APMServerBehavior: TimelyResponse, ExecutionDuration: 1, Timeout: 5},
	}
	eventQueueGenerator(eventsChain, suite.eventsChannel)
	assert.NotPanics(suite.T(), main)
	assert.True(suite.T(), strings.Contains(suite.apmServerInternals.Data, string(TimelyResponse)))
}

// TestFlush checks if the flushed param does not cause a panic or an unexpected behavior
func (suite *MainUnitTestsSuite) TestFlush() {
	eventsChain := []MockEvent{
		{Type: InvokeStandardFlush, APMServerBehavior: TimelyResponse, ExecutionDuration: 1, Timeout: 5},
	}
	eventQueueGenerator(eventsChain, suite.eventsChannel)
	assert.NotPanics(suite.T(), main)
	assert.True(suite.T(), strings.Contains(suite.apmServerInternals.Data, string(TimelyResponse)))
}

// TestWaitGroup checks if there is no race condition between the main waitgroups (issue #128)
func (suite *MainUnitTestsSuite) TestWaitGroup() {
	eventsChain := []MockEvent{
		{Type: InvokeWaitgroupsRace, APMServerBehavior: TimelyResponse, ExecutionDuration: 1, Timeout: 500},
	}
	eventQueueGenerator(eventsChain, suite.eventsChannel)
	assert.NotPanics(suite.T(), main)
	assert.True(suite.T(), strings.Contains(suite.apmServerInternals.Data, string(TimelyResponse)))
}

// TestAPMServerDown tests that main does not panic nor runs indefinitely when the APM server is inactive.
func (suite *MainUnitTestsSuite) TestAPMServerDown() {
	suite.apmServer.Close()
	eventsChain := []MockEvent{
		{Type: InvokeStandard, APMServerBehavior: TimelyResponse, ExecutionDuration: 1, Timeout: 5},
	}
	eventQueueGenerator(eventsChain, suite.eventsChannel)
	assert.NotPanics(suite.T(), main)
	assert.False(suite.T(), strings.Contains(suite.apmServerInternals.Data, string(TimelyResponse)))
}

// TestAPMServerHangs tests that main does not panic nor runs indefinitely when the APM server does not respond.
func (suite *MainUnitTestsSuite) TestAPMServerHangs() {
	eventsChain := []MockEvent{
		{Type: InvokeStandard, APMServerBehavior: Hangs, ExecutionDuration: 1, Timeout: 5},
	}
	eventQueueGenerator(eventsChain, suite.eventsChannel)
	assert.NotPanics(suite.T(), main)
	assert.False(suite.T(), strings.Contains(suite.apmServerInternals.Data, string(Hangs)))
	suite.apmServerInternals.UnlockSignalChannel <- struct{}{}
}

// TestAPMServerRecovery tests a scenario where the APM server recovers after hanging.
// The default forwarder timeout is 3 seconds, so we wait 5 seconds until we unlock that hanging state.
// Hence, the APM server is waked up just in time to process the TimelyResponse data frame.
func (suite *MainUnitTestsSuite) TestAPMServerRecovery() {
	if err := os.Setenv("ELASTIC_APM_DATA_FORWARDER_TIMEOUT_SECONDS", "1"); err != nil {
		suite.T().Fail()
	}
	if err := os.Setenv("ELASTIC_APM_LOG_LEVEL", "trace"); err != nil {
		suite.T().Fail()
	}

	eventsChain := []MockEvent{
		{Type: InvokeStandard, APMServerBehavior: Hangs, ExecutionDuration: 1, Timeout: 5},
		{Type: InvokeStandard, APMServerBehavior: TimelyResponse, ExecutionDuration: 1, Timeout: 5},
	}
	eventQueueGenerator(eventsChain, suite.eventsChannel)
	go func() {
		time.Sleep(2500 * time.Millisecond) // Cannot multiply time.Second by a float
		suite.apmServerInternals.UnlockSignalChannel <- struct{}{}
	}()
	assert.NotPanics(suite.T(), main)
	assert.True(suite.T(), strings.Contains(suite.apmServerInternals.Data, string(Hangs)))
	assert.True(suite.T(), strings.Contains(suite.apmServerInternals.Data, string(TimelyResponse)))
	if err := os.Setenv("ELASTIC_APM_DATA_FORWARDER_TIMEOUT_SECONDS", ""); err != nil {
		suite.T().Fail()
	}
}

// TestGracePeriodHangs verifies that the WaitforGracePeriod goroutine ends when main() ends.
// This can be checked by asserting that apmTransportStatus is pending after the execution of main.
func (suite *MainUnitTestsSuite) TestGracePeriodHangs() {
	extension.ApmServerTransportState.Status = extension.Pending
	extension.ApmServerTransportState.ReconnectionCount = 100

	eventsChain := []MockEvent{
		{Type: InvokeStandard, APMServerBehavior: Hangs, ExecutionDuration: 1, Timeout: 5},
	}
	eventQueueGenerator(eventsChain, suite.eventsChannel)
	assert.NotPanics(suite.T(), main)

	time.Sleep(100 * time.Millisecond)
	suite.apmServerInternals.UnlockSignalChannel <- struct{}{}
	defer assert.Equal(suite.T(), extension.IsTransportStatusHealthyOrPending(), true)
}

// TestAPMServerCrashesDuringExecution tests that main does not panic nor runs indefinitely when the APM server crashes
// during execution.
func (suite *MainUnitTestsSuite) TestAPMServerCrashesDuringExecution() {
	eventsChain := []MockEvent{
		{Type: InvokeStandard, APMServerBehavior: Crashes, ExecutionDuration: 1, Timeout: 5},
	}
	eventQueueGenerator(eventsChain, suite.eventsChannel)
	assert.NotPanics(suite.T(), main)
	assert.False(suite.T(), strings.Contains(suite.apmServerInternals.Data, string(Crashes)))
}

// TestFullChannel checks that an overload of APM data chunks is handled correctly, events dropped beyond the 100th one
// if no space left in channel, no panic, no infinite hanging.
func (suite *MainUnitTestsSuite) TestFullChannel() {
	eventsChain := []MockEvent{
		{Type: InvokeMultipleTransactionsOverload, APMServerBehavior: TimelyResponse, ExecutionDuration: 0.1, Timeout: 5},
	}
	eventQueueGenerator(eventsChain, suite.eventsChannel)
	assert.NotPanics(suite.T(), main)
	assert.True(suite.T(), strings.Contains(suite.apmServerInternals.Data, string(TimelyResponse)))
}

// TestFullChannelSlowAPMServer tests what happens when the APM Data channel is full and the APM server is slow
// (send strategy : background)
func (suite *MainUnitTestsSuite) TestFullChannelSlowAPMServer() {
	if err := os.Setenv("ELASTIC_APM_SEND_STRATEGY", "background"); err != nil {
		suite.T().Fail()
	}

	eventsChain := []MockEvent{
		{Type: InvokeMultipleTransactionsOverload, APMServerBehavior: SlowResponse, ExecutionDuration: 0.01, Timeout: 5},
	}
	eventQueueGenerator(eventsChain, suite.eventsChannel)
	assert.NotPanics(suite.T(), main)
	// The test should not hang
	if err := os.Setenv("ELASTIC_APM_SEND_STRATEGY", "syncflush"); err != nil {
		suite.T().Fail()
	}
}
