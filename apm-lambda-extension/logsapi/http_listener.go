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
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/pkg/errors"
)

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

// LogsAPIHttpListener is used to listen to the Logs API using HTTP
type LogsAPIHttpListener struct {
	httpServer *http.Server

	logChannel chan LogEvent
}

// NewLogsAPIHttpListener returns a LogsAPIHttpListener with the given log queue
func NewLogsAPIHttpListener(lc chan LogEvent) (*LogsAPIHttpListener, error) {

	return &LogsAPIHttpListener{
		httpServer: nil,
		logChannel: lc,
	}, nil
}

func ListenOnAddress() string {
	env_aws_local, ok := os.LookupEnv("AWS_SAM_LOCAL")
	if ok && env_aws_local == "true" {
		return ":" + DefaultHttpListenerPort
	}

	listenerAddress, ok := os.LookupEnv("ELASTIC_APM_LAMBDA_LOGS_LISTENER_ADDRESS")
	if ok && listenerAddress != "" {
		return listenerAddress
	}
	return "sandbox:" + DefaultHttpListenerPort
}

// Start initiates the server in a goroutine where the logs will be sent
func (s *LogsAPIHttpListener) Start(address string) (bool, error) {
	s.httpServer = &http.Server{Addr: address}
	http.HandleFunc("/", s.http_handler)
	go func() {
		log.Printf("Server listening for logs data from AWS Logs API on %s", address)
		err := s.httpServer.ListenAndServe()
		if err != http.ErrServerClosed {
			log.Printf("Unexpected stop on Logs API Http Server: %v", err)
			s.Shutdown()
		} else {
			log.Printf("Logs API Http Server closed %v", err)
		}
	}()
	return true, nil
}

func (le *LogEvent) unmarshalRecord() error {
	if SubEventType(le.Type) != Fault {
		record := LogEventRecord{}
		err := json.Unmarshal([]byte(le.RawRecord), &record)
		if err != nil {
			return errors.New("Could not unmarshal log event raw record into record")
		}
		le.Record = record
	}
	return nil
}

// http_handler handles the requests coming from the Logs API.
// Everytime Logs API sends logs, this function will read the logs from the response body
// and put them into a synchronous queue to be read by the main goroutine.
// Logging or printing besides the error cases below is not recommended if you have subscribed to receive extension logs.
// Otherwise, logging here will cause Logs API to send new logs for the printed lines which will create an infinite loop.
func (h *LogsAPIHttpListener) http_handler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body of Logs API request: %+v", err)
		return
	}

	var logEvents []LogEvent
	err = json.Unmarshal(body, &logEvents)
	if err != nil {
		log.Println("error unmarshaling log event:", err)
	}

	for idx := range logEvents {
		err = logEvents[idx].unmarshalRecord()
		if err != nil {
			log.Printf("Error unmarshalling log event: %+v", err)
		}
		h.logChannel <- logEvents[idx]
	}
}

func (s *LogsAPIHttpListener) Shutdown() {
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		err := s.httpServer.Shutdown(ctx)
		if err != nil {
			log.Printf("Failed to shutdown Logs API http server gracefully %s", err)
		} else {
			s.httpServer = nil
		}
	}
}
