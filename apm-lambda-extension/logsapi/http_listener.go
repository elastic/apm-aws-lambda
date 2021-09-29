// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT-0

package logsapi

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

type LogEvent struct {
	Time   time.Time      `json:"time"`
	Type   string         `json:"type"`
	Record LogEventRecord `json:"record"`
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
	if ok && "true" == env_aws_local {
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
			log.Printf("Unexpected stop on Http Server: %v", err)
			s.Shutdown()
		} else {
			log.Printf("Http Server closed %v", err)
		}
	}()
	return true, nil
}

// http_handler handles the requests coming from the Logs API.
// Everytime Logs API sends logs, this function will read the logs from the response body
// and put them into a synchronous queue to be read by the main goroutine.
// Logging or printing besides the error cases below is not recommended if you have subscribed to receive extension logs.
// Otherwise, logging here will cause Logs API to send new logs for the printed lines which will create an infinite loop.
func (h *LogsAPIHttpListener) http_handler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body: %+v", err)
		return
	}

	var logEvents []LogEvent
	err = json.Unmarshal(body, &logEvents)
	if err != nil {
		log.Println("error unmarshaling log event:", err)
	}
	// Send the log events to the channel
	for _, logEvent := range logEvents {
		h.logChannel <- logEvent
	}
}

func (s *LogsAPIHttpListener) Shutdown() {
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		err := s.httpServer.Shutdown(ctx)
		if err != nil {
			log.Printf("Failed to shutdown http server gracefully %s", err)
		} else {
			s.httpServer = nil
		}
	}
}
