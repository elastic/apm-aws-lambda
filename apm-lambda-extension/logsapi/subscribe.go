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
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/pkg/errors"
)

const DefaultHttpListenerPort = "0"

var logsAPIServer *http.Server
var logsAPIListener net.Listener

type LogEvent struct {
	Time      time.Time       `json:"time"`
	Type      string          `json:"type"`
	RawRecord json.RawMessage `json:"record"`
	Record    LogEventRecord
}

type LogEventRecord struct {
	RequestId string `json:"requestId"`
	Status    string `json:"status"`
}

// Init initializes the configuration for the Logs API and subscribes to the Logs API for HTTP
func subscribe(extensionID string, eventTypes []EventType) error {
	extensions_api_address, ok := os.LookupEnv("AWS_LAMBDA_RUNTIME_API")
	if !ok {
		return errors.New("AWS_LAMBDA_RUNTIME_API is not set")
	}

	logsAPIBaseUrl := fmt.Sprintf("http://%s", extensions_api_address)
	logsAPIClient, err := NewClient(logsAPIBaseUrl)
	if err != nil {
		return err
	}

	bufferingCfg := BufferingCfg{
		MaxItems:  10000,
		MaxBytes:  262144,
		TimeoutMS: 25,
	}

	destination := Destination{
		Protocol:   HttpProto,
		URI:        URI("http://" + logsAPIListener.Addr().String()),
		HttpMethod: HttpPost,
		Encoding:   JSON,
	}

	_, err = logsAPIClient.Subscribe(eventTypes, bufferingCfg, destination, extensionID)
	return err
}

func Subscribe(ctx context.Context, extensionID string, eventTypes []EventType, out chan LogEvent) error {
	if checkAwsSamLocal() {
		// TODO: or return error?
		log.Printf("Not subscribing to LogsAPI, detected sam local environment")
		return nil
	} else {
		err := startHttpServer(out)
		if err != nil {
			return err
		}

		err = subscribe(extensionID, eventTypes)
		if err != nil {
			return err
		}
	}
	return nil
}

func startHttpServer(out chan LogEvent) (err error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleLogEventsRequest(out))

	listenerAddress, ok := os.LookupEnv("ELASTIC_APM_LAMBDA_LOGS_LISTENER_ADDRESS")
	if !ok || listenerAddress == "" {
		listenerAddress = "localhost:0"
	}

	logsAPIServer = &http.Server{
		Handler: mux,
	}

	logsAPIListener, err = net.Listen("tcp", listenerAddress)
	if err != nil {
		return
	}

	go func() {
		log.Printf("Extension listening for logsAPI events on %s", logsAPIListener.Addr())
		logsAPIServer.Serve(logsAPIListener)
	}()
	return nil
}

func checkAwsSamLocal() bool {
	env_aws_local, ok := os.LookupEnv("ELASTIC_APM_LAMBDA_LOGS_LISTENER_ADDRESS")
	if ok && env_aws_local == "true" {
		return true
	}

	return false
}
