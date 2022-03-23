// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
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

var ListenerHost = "sandbox"
var Server *http.Server
var Listener net.Listener

type LogEvent struct {
	Time         time.Time    `json:"time"`
	Type         SubEventType `json:"type"`
	StringRecord string
	Record       LogEventRecord
}

type LogEventRecord struct {
	RequestId string          `json:"requestId"`
	Status    string          `json:"status"`
	Metrics   PlatformMetrics `json:"metrics"`
}

type PlatformMetrics struct {
	DurationMs       float32 `json:"durationMs"`
	BilledDurationMs int32   `json:"billedDurationMs"`
	MemorySizeMB     int32   `json:"memorySizeMB"`
	MaxMemoryUsedMB  int32   `json:"maxMemoryUsedMB"`
	InitDurationMs   float32 `json:"initDurationMs"`
}

// Subscribes to the Logs API
func subscribe(extensionID string, eventTypes []EventType) error {
	extensionsAPIAddress, ok := os.LookupEnv("AWS_LAMBDA_RUNTIME_API")
	if !ok {
		return errors.New("AWS_LAMBDA_RUNTIME_API is not set")
	}

	logsAPIBaseUrl := fmt.Sprintf("http://%s", extensionsAPIAddress)
	logsAPIClient, err := NewClient(logsAPIBaseUrl)
	if err != nil {
		return err
	}

	_, port, _ := net.SplitHostPort(Listener.Addr().String())
	_, err = logsAPIClient.Subscribe(eventTypes, URI("http://"+ListenerHost+":"+port), extensionID)
	return err
}

// Starts the HTTP server listening for log events and subscribes to the Logs API
func Subscribe(ctx context.Context, extensionID string, eventTypes []EventType, out chan LogEvent) (err error) {
	if checkAWSSamLocal() {
		return errors.New("Detected sam local environment")
	}
	err = startHTTPServer(out)
	if err != nil {
		return
	}

	err = subscribe(extensionID, eventTypes)
	if err != nil {
		return
	}
	return nil
}

func startHTTPServer(out chan LogEvent) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleLogEventsRequest(out))
	var err error

	Server = &http.Server{
		Handler: mux,
	}

	Listener, err = net.Listen("tcp", ListenerHost+":0")
	if err != nil {
		return err
	}

	go func() {
		extension.Log.Infof("Extension listening for Lambda Logs API events on %s", Listener.Addr().String())
		Server.Serve(Listener)
	}()
	return nil
}

func checkAWSSamLocal() bool {
	envAWSLocal, ok := os.LookupEnv("AWS_SAM_LOCAL")
	if ok && envAWSLocal == "true" {
		return true
	}

	return false
}
