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
	"time"
)

// EventType represents the type of logs in Lambda
type EventType string

const (
	// Platform is to receive logs emitted by the platform
	Platform EventType = "platform"
	// Function is to receive logs emitted by the function
	Function EventType = "function"
	// Extension is to receive logs emitted by the extension
	Extension EventType = "extension"
)

// SubEventType is a Logs API sub event type
type SubEventType string

const (
	// RuntimeDone event is sent when lambda function is finished it's execution
	RuntimeDone SubEventType = "platform.runtimeDone"
	Fault       SubEventType = "platform.fault"
	Report      SubEventType = "platform.report"
	Start       SubEventType = "platform.start"
)

// LogEvent represents an event received from the Logs API
type LogEvent struct {
	Time         time.Time    `json:"time"`
	Type         SubEventType `json:"type"`
	StringRecord string
	Record       LogEventRecord
}

// LogEventRecord is a sub-object in a Logs API event
type LogEventRecord struct {
	RequestID string          `json:"requestId"`
	Status    string          `json:"status"`
	Metrics   PlatformMetrics `json:"metrics"`
}

// ProcessLogs consumes events until a RuntimeDone event corresponding
// to requestID is received, or ctx is canceled, and then returns.
func (lc *Client) ProcessLogs(
	ctx context.Context,
	requestID string,
	apmServerTransport *extension.ApmServerTransport,
	metadataContainer *extension.MetadataContainer,
	runtimeDoneSignal chan struct{},
	prevEvent *extension.NextEventResponse,
) error {
	for {
		select {
		case logEvent := <-lc.logsChannel:
			extension.Log.Debugf("Received log event %v", logEvent.Type)
			switch logEvent.Type {
			// Check the logEvent for runtimeDone and compare the RequestID
			// to the id that came in via the Next API
			case RuntimeDone:
				if logEvent.Record.RequestID == requestID {
					extension.Log.Info("Received runtimeDone event for this function invocation")
					runtimeDoneSignal <- struct{}{}
					return nil
				}

				extension.Log.Debug("Log API runtimeDone event request id didn't match")
			// Check if the logEvent contains metrics and verify that they can be linked to the previous invocation
			case Report:
				if prevEvent != nil && logEvent.Record.RequestID == prevEvent.RequestID {
					extension.Log.Debug("Received platform report for the previous function invocation")
					processedMetrics, err := ProcessPlatformReport(metadataContainer, prevEvent, logEvent)
					if err != nil {
						extension.Log.Errorf("Error processing Lambda platform metrics : %v", err)
					} else {
						apmServerTransport.EnqueueAPMData(processedMetrics)
					}
				} else {
					extension.Log.Warn("report event request id didn't match the previous event id")
					extension.Log.Debug("Log API runtimeDone event request id didn't match")
				}
			}
		case <-ctx.Done():
			extension.Log.Debug("Current invocation over. Interrupting logs processing goroutine")
			return nil
		}
	}
}
