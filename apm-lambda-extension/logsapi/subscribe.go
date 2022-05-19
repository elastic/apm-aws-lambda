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

package logsapi

import (
	"context"
	"elastic/apm-lambda-extension/extension"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/pkg/errors"
)

// TODO: Remove global variable and find another way to retrieve Logs Listener network info when testing main
// TestListenerAddr For e2e testing purposes
var TestListenerAddr net.Addr

type LogsTransport struct {
	logsChannel  chan LogEvent
	listener     net.Listener
	listenerHost string
	server       *http.Server
}

func InitLogsTransport(listenerHost string) *LogsTransport {
	var transport LogsTransport
	transport.listenerHost = listenerHost
	transport.logsChannel = make(chan LogEvent, 100)
	return &transport
}

// LogEvent represents an event received from the Logs API
type LogEvent struct {
	Time         time.Time    `json:"time"`
	Type         SubEventType `json:"type"`
	StringRecord string
	Record       LogEventRecord
}

// LogEventRecord is a sub-object in a Logs API event
type LogEventRecord struct {
	RequestId string `json:"requestId"`
	Status    string `json:"status"`
}

// Subscribes to the Logs API
func subscribe(transport *LogsTransport, extensionID string, eventTypes []EventType) error {

	extensionsAPIAddress, ok := os.LookupEnv("AWS_LAMBDA_RUNTIME_API")
	if !ok {
		return errors.New("AWS_LAMBDA_RUNTIME_API is not set")
	}

	logsAPIBaseUrl := fmt.Sprintf("http://%s", extensionsAPIAddress)
	logsAPIClient, err := NewClient(logsAPIBaseUrl)
	if err != nil {
		return err
	}

	_, port, _ := net.SplitHostPort(transport.listener.Addr().String())
	_, err = logsAPIClient.Subscribe(eventTypes, URI("http://"+transport.listenerHost+":"+port), extensionID)
	return err
}

// Subscribe starts the HTTP server listening for log events and subscribes to the Logs API
func Subscribe(ctx context.Context, extensionID string, eventTypes []EventType) (transport *LogsTransport, err error) {
	if checkAWSSamLocal() {
		return nil, errors.New("Detected sam local environment")
	}

	// Init APM server Transport struct
	// Make channel for collecting logs and create a HTTP server to listen for them
	if checkLambdaFunction() {
		transport = InitLogsTransport("sandbox")
	} else {
		transport = InitLogsTransport("localhost")
	}

	if err = startHTTPServer(ctx, transport); err != nil {
		return nil, err
	}

	if err = subscribe(transport, extensionID, eventTypes); err != nil {
		return nil, err
	}
	return transport, nil
}

func startHTTPServer(ctx context.Context, transport *LogsTransport) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleLogEventsRequest(transport))
	var err error

	transport.server = &http.Server{
		Handler: mux,
	}

	if transport.listener, err = net.Listen("tcp", transport.listenerHost+":0"); err != nil {
		return err
	}
	TestListenerAddr = transport.listener.Addr()

	go func() {
		extension.Log.Infof("Extension listening for Lambda Logs API events on %s", transport.listener.Addr().String())
		if err = transport.server.Serve(transport.listener); err != nil {
			extension.Log.Errorf("Error upon Logs API server start : %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		transport.server.Close()
	}()

	return nil
}

// checkAWSSamLocal checks if the extension is running in a SAM CLI container.
// The Logs API is not supported in that scenario.
func checkAWSSamLocal() bool {
	envAWSLocal, ok := os.LookupEnv("AWS_SAM_LOCAL")
	if ok && envAWSLocal == "true" {
		return true
	}

	return false
}

// checkLambdaFunction checks if the extension is together with an actual Lambda function.
// It is currently used together with checkAWSSamLocal as a best effort solution to determine if
// the extension actually runs in dev (unit tests), SAM, or in a local Lambda environment.
func checkLambdaFunction() bool {
	_, ok := os.LookupEnv("AWS_LAMBDA_FUNCTION_NAME")
	if ok {
		return true
	}

	return false
}

// WaitRuntimeDone consumes events until a RuntimeDone event corresponding
// to requestID is received, or ctx is cancelled, and then returns.
func WaitRuntimeDone(ctx context.Context, requestID string, transport *LogsTransport, runtimeDoneSignal chan struct{}) error {
	for {
		select {
		case logEvent := <-transport.logsChannel:
			extension.Log.Debugf("Received log event %v", logEvent.Type)
			// Check the logEvent for runtimeDone and compare the RequestID
			// to the id that came in via the Next API
			if logEvent.Type == RuntimeDone {
				if logEvent.Record.RequestId == requestID {
					extension.Log.Info("Received runtimeDone event for this function invocation")
					runtimeDoneSignal <- struct{}{}
					return nil
				} else {
					extension.Log.Debug("Log API runtimeDone event request id didn't match")
				}
			}
		case <-ctx.Done():
			extension.Log.Debug("Current invocation over. Interrupting logs processing goroutine")
			return nil
		}
	}
}
