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
)

// LogEventType represents the log type that is received in the log messages
type LogEventType string

type Forwarder func(context.Context, []byte) error

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

// ProcessLogs consumes log events until there are no more log events that
// can be consumed or ctx is canceled. For INVOKE event this state is
// reached when runtimeDone event for the current requestID is processed
// whereas for SHUTDOWN event this state is reached when the platformReport
// event for the previous requestID is processed.
func (lc *Client) ProcessLogs(
	ctx context.Context,
	requestID string,
	invokedFnArn string,
	forwarFn Forwarder,
	isShutdown bool,
) {
	for {
		select {
		case logEvent := <-lc.logsChannel:
			if shouldExit := lc.handleEvent(ctx, logEvent, requestID, invokedFnArn, forwarFn, isShutdown); shouldExit {
				return
			}
		case <-ctx.Done():
			lc.logger.Debug("Current invocation over. Interrupting logs processing goroutine")
			return
		}
	}
}

func (lc *Client) FlushData(
	ctx context.Context,
	requestID string,
	invokedFnArn string,
	forwardFn Forwarder,
	isShutdown bool,
) {
	lc.logger.Infof("flushing %d buffered logs", len(lc.logsChannel))
	for {
		select {
		case logEvent := <-lc.logsChannel:
			if shouldExit := lc.handleEvent(ctx, logEvent, requestID, invokedFnArn, forwardFn, isShutdown); shouldExit {
				return
			}
		case <-ctx.Done():
			lc.logger.Debug("Current invocation over. Interrupting logs flushing")
			return
		default:
			if len(lc.logsChannel) == 0 {
				lc.logger.Debug("Flush ended for logs - no data in buffer")
				return
			}
		}
	}
}

func (lc *Client) handleEvent(ctx context.Context,
	logEvent LogEvent,
	requestID string,
	invokedFnArn string,
	forwardFn Forwarder,
	isShutdown bool,
) bool {
	lc.logger.Debugf("Received log event %v for request ID %s", logEvent.Type, logEvent.Record.RequestID)
	switch logEvent.Type {
	case PlatformStart:
		lc.invocationLifecycler.OnPlatformStart(logEvent.Record.RequestID)
	case PlatformRuntimeDone:
		if err := lc.invocationLifecycler.OnLambdaLogRuntimeDone(
			logEvent.Record.RequestID,
			logEvent.Record.Status,
			logEvent.Time,
		); err != nil {
			lc.logger.Warnf("Failed to finalize invocation with request ID %s: %v", logEvent.Record.RequestID, err)
		}
		// For invocation events the platform.runtimeDone would be the last possible event.
		if !isShutdown && logEvent.Record.RequestID == requestID {
			lc.logger.Debugf(
				"Processed runtime done event for reqID %s as the last log event for the invocation",
				logEvent.Record.RequestID,
			)
			return true
		}
	case PlatformReport:
		fnARN, deadlineMs, ts, err := lc.invocationLifecycler.OnPlatformReport(logEvent.Record.RequestID)
		if err != nil {
			lc.logger.Warnf("Failed to process platform report: %v", err)
		} else {
			lc.logger.Debugf("Received platform report for %s", logEvent.Record.RequestID)
			processedMetrics, err := ProcessPlatformReport(fnARN, deadlineMs, ts, logEvent)
			if err != nil {
				lc.logger.Errorf("Error processing Lambda platform metrics: %v", err)
			} else {
				if err := forwardFn(ctx, processedMetrics); err != nil {
					lc.logger.Errorf("Error forwarding Lambda platform metrics: %v", err)
				}
			}
		}
		// For shutdown event the platform report metrics for the previous log event
		// would be the last possible log event. After processing this metric the
		// invocation lifecycler's cache should be empty.
		if isShutdown && lc.invocationLifecycler.Size() == 0 {
			lc.logger.Debugf(
				"Processed platform report event for reqID %s as the last log event before shutdown",
				logEvent.Record.RequestID,
			)
			return true
		}
	case PlatformLogsDropped:
		lc.logger.Warnf("Logs dropped due to extension falling behind: %v", logEvent.Record)
	case FunctionLog:
		processedLog, err := ProcessFunctionLog(
			lc.invocationLifecycler.PlatformStartReqID(),
			invokedFnArn,
			logEvent,
		)
		if err != nil {
			lc.logger.Warnf("Error processing function log : %v", err)
		} else {
			if err := forwardFn(ctx, processedLog); err != nil {
				lc.logger.Warnf("Error forwarding function log : %v", err)
			}
		}
	}
	return false
}
