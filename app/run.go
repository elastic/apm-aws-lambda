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

package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/elastic/apm-aws-lambda/extension"
)

// Run runs the app.
func (app *App) Run(ctx context.Context) error {
	// register extension with AWS Extension API
	res, err := app.extensionClient.Register(ctx, app.extensionName)
	if err != nil {
		app.logger.Errorf("Error: %s", err)

		status, errRuntime := app.extensionClient.InitError(ctx, err.Error())
		if errRuntime != nil {
			return errRuntime
		}

		app.logger.Infof("Init error signal sent to runtime : %s", status)
		app.logger.Infof("Exiting")
		return err
	}
	app.logger.Debugf("Register response: %v", extension.PrettyPrint(res))

	// start http server to receive data from agent
	err = app.apmClient.StartReceiver()
	if err != nil {
		return fmt.Errorf("failed to start the APM data receiver : %w", err)
	}
	defer func() {
		if err := app.apmClient.Shutdown(); err != nil {
			app.logger.Warnf("Error while shutting down the apm receiver: %v", err)
		}
	}()

	// Flush all data before shutting down.
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		app.apmClient.FlushAPMData(ctx)
	}()

	if app.logsClient != nil {
		if err := app.logsClient.StartService(app.extensionClient.ExtensionID); err != nil {
			app.logger.Warnf("Error while subscribing to the Logs API: %v", err)

			// disable logs API if the service failed to start
			app.logsClient = nil
		} else {
			// Remember to shutdown the log service if available.
			defer func() {
				if err := app.logsClient.Shutdown(); err != nil {
					app.logger.Warnf("failed to shutdown the log service: %v", err)
				}
			}()
		}
	}

	for {
		select {
		case <-ctx.Done():
			app.logger.Info("Received a signal, exiting...")
			return nil
		default:
			// Use a wait group to ensure the background go routine sending to the APM server
			// completes before signaling that the extension is ready for the next invocation.
			var backgroundDataSendWg sync.WaitGroup
			event, err := app.processEvent(ctx, &backgroundDataSendWg)
			if err != nil {
				return err
			}
			app.logger.Debug("Waiting for background data send to end")
			backgroundDataSendWg.Wait()
			if event.EventType == extension.Shutdown {
				app.logger.Infof("Exiting due to shutdown event with reason %s", event.ShutdownReason)
				// Since we have waited for the processEvent loop to finish we
				// already have received all the data we can from the agent. So, we
				// flush all the data to make sure that shutdown can correctly deduce
				// any pending transactions.
				app.apmClient.FlushAPMData(ctx)
				// At shutdown we can not expect platform.runtimeDone events to be
				// reported for the remaining invocations. If we haven't received the
				// transaction from agents at this point then it is safe to assume
				// that the function failed. We will create proxy transaction for all
				// invocations that haven't received a full transaction from the agent
				// yet. If extension doesn't have enough CPU time it is possible that
				// the extension might not receive the shutdown signal for timeouts
				// or runtime crashes. In these cases we will miss the transaction.
				if err := app.batch.OnShutdown(event.ShutdownReason); err != nil {
					app.logger.Errorf("Error finalizing invocation on shutdown: %v", err)
				}
				return nil
			}
			if app.apmClient.ShouldFlush() {
				// Use a new cancellable context for flushing APM data to make sure
				// that the underlying transport is reset for next invocation without
				// waiting for grace period if it got to unhealthy state.
				flushCtx, cancel := context.WithCancel(ctx)
				// Flush APM data now that the function invocation has completed
				app.apmClient.FlushAPMData(flushCtx)
				cancel()
			}
		}
	}
}

func (app *App) processEvent(
	ctx context.Context,
	backgroundDataSendWg *sync.WaitGroup,
) (*extension.NextEventResponse, error) {
	// Reset flush state for future events.
	defer app.apmClient.ResetFlush()

	// Invocation context
	invocationCtx, invocationCancel := context.WithCancel(ctx)
	defer invocationCancel()

	// call Next method of extension API.  This long polling HTTP method
	// will block until there's an invocation of the function
	app.logger.Info("Waiting for next event...")
	event, err := app.extensionClient.NextEvent(ctx)
	if err != nil {
		app.logger.Errorf("Error: %s", err)

		status, errRuntime := app.extensionClient.ExitError(ctx, err.Error())
		if errRuntime != nil {
			return nil, errRuntime
		}

		app.logger.Infof("Exit signal sent to runtime : %s", status)
		app.logger.Info("Exiting")
		return nil, err
	}

	// Used to compute Lambda Timeout
	event.Timestamp = time.Now()
	app.logger.Debug("Received event.")
	app.logger.Debugf("%v", extension.PrettyPrint(event))

	switch event.EventType {
	case extension.Invoke:
		app.batch.RegisterInvocation(
			event.RequestID,
			event.InvokedFunctionArn,
			event.DeadlineMs,
			event.Timestamp,
		)
	case extension.Shutdown:
		// platform.report metric (and some other metrics) might not have been
		// reported by the logs API even till shutdown. At shutdown we will make
		// a last attempt to collect and report these metrics. However, it is
		// also possible that lambda has init a few execution env preemptively,
		// for such cases the extension will see only a SHUTDOWN event and
		// there is no need to wait for any log event.
		if app.batch.Size() == 0 {
			return event, nil
		}
	}

	// APM Data Processing
	backgroundDataSendWg.Add(1)
	go func() {
		defer backgroundDataSendWg.Done()
		if err := app.apmClient.ForwardApmData(invocationCtx); err != nil {
			app.logger.Error(err)
		}
	}()

	// Lambda Service Logs Processing, also used to extract metrics from APM logs
	// This goroutine should not be started if subscription failed
	logProcessingDone := make(chan struct{})
	if app.logsClient != nil {
		go func() {
			defer close(logProcessingDone)
			app.logsClient.ProcessLogs(
				invocationCtx,
				event.RequestID,
				event.InvokedFunctionArn,
				app.apmClient.LambdaDataChannel,
				event.EventType == extension.Shutdown,
			)
		}()
	} else {
		app.logger.Warn("Logs collection not started due to earlier subscription failure")
		close(logProcessingDone)
	}

	// Calculate how long to wait for a runtimeDoneSignal or AgentDoneSignal signal
	flushDeadlineMs := event.DeadlineMs - 200
	durationUntilFlushDeadline := time.Until(time.Unix(flushDeadlineMs/1000, 0))

	// Create a timer that expires after durationUntilFlushDeadline
	timer := time.NewTimer(durationUntilFlushDeadline)
	defer timer.Stop()

	// The extension relies on 3 independent mechanisms to minimize the time interval
	// between the end of the execution of the lambda function and the end of the
	// execution of processEvent():
	// 1) AgentDoneSignal triggered upon reception of a `flushed=true` query from the agent
	// 2) [Backup 1] All expected log events are processed.
	// 3) [Backup 2] If all else fails, the extension relies of the timeout of the Lambda
	// function to interrupt itself 200ms before the specified deadline to give the extension
	// time to flush data before shutdown.
	select {
	case <-app.apmClient.WaitForFlush():
		app.logger.Debug("APM client has sent flush signal")
	case <-logProcessingDone:
		app.logger.Debug("Received runtimeDone signal")
	case <-timer.C:
		app.logger.Info("Time expired while waiting for agent done signal or final log event")
	}
	return event, nil
}
