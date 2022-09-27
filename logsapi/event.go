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
	"time"

	"github.com/elastic/apm-aws-lambda/apmproxy"
	"github.com/elastic/apm-aws-lambda/extension"
)

// LogEventType represents the log type that is received in the log messages
type LogEventType string

const (
	// PlatformRuntimeDone event is sent when lambda function is finished it's execution
	PlatformRuntimeDone LogEventType = "platform.runtimeDone"
	PlatformFault       LogEventType = "platform.fault"
	PlatformReport      LogEventType = "platform.report"
	PlatformLogsDropped LogEventType = "platform.logsDropped"
	PlatformStart       LogEventType = "platform.start"
	PlatformEnd         LogEventType = "platform.end"
	FunctionLog         LogEventType = "function"
)

// LogEvent represents an event received from the Logs API
type LogEvent struct {
	Time         time.Time    `json:"time"`
	Type         LogEventType `json:"type"`
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
	invokedFnArn string,
	apmClient *apmproxy.Client,
	runtimeDoneSignal chan struct{},
	prevEvent *extension.NextEventResponse,
) error {
	// platformStartReqID is to identify the requestID for the function
	// logs under the assumption that function logs for a specific request
	// ID will be bounded by PlatformStart and PlatformEnd events.
	var platformStartReqID string
	for {
		select {
		case logEvent := <-lc.logsChannel:
			lc.logger.Debugf("Received log event %v", logEvent.Type)
			switch logEvent.Type {
			case PlatformStart:
				platformStartReqID = logEvent.Record.RequestID
			// Check the logEvent for runtimeDone and compare the RequestID
			// to the id that came in via the Next API
			case PlatformRuntimeDone:
				if logEvent.Record.RequestID == requestID {
					lc.logger.Info("Received runtimeDone event for this function invocation")
					runtimeDoneSignal <- struct{}{}
					return nil
				}

				lc.logger.Debug("Log API runtimeDone event request id didn't match")
			// Check if the logEvent contains metrics and verify that they can be linked to the previous invocation
			case PlatformReport:
				if prevEvent != nil && logEvent.Record.RequestID == prevEvent.RequestID {
					lc.logger.Debug("Received platform report for the previous function invocation")
					processedMetrics, err := ProcessPlatformReport(prevEvent, logEvent)
					if err != nil {
						lc.logger.Errorf("Error processing Lambda platform metrics: %v", err)
					} else {
						apmClient.EnqueueAPMData(processedMetrics)
					}
				} else {
					lc.logger.Warn("report event request id didn't match the previous event id")
					lc.logger.Debug("Log API runtimeDone event request id didn't match")
				}
			case PlatformLogsDropped:
				lc.logger.Warn("Logs dropped due to extension falling behind: %v", logEvent.Record)
			case FunctionLog:
				lc.logger.Debug("Received function log")
				processedLog, err := ProcessFunctionLog(
					platformStartReqID,
					invokedFnArn,
					logEvent,
				)
				if err != nil {
					lc.logger.Errorf("Error processing function log : %v", err)
				} else {
					apmClient.EnqueueAPMData(processedLog)
				}
			}
		case <-ctx.Done():
			lc.logger.Debug("Current invocation over. Interrupting logs processing goroutine")
			return nil
		}
	}
}
