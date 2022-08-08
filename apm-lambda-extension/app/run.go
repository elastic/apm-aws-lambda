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
	"elastic/apm-lambda-extension/apmproxy"
	"elastic/apm-lambda-extension/extension"
	"elastic/apm-lambda-extension/logsapi"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// Run runs the app.
func (app *App) Run(ctx context.Context) error {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS default config: %w", err)
	}
	manager := secretsmanager.NewFromConfig(cfg)
	// pulls ELASTIC_ env variable into globals for easy access
	config := extension.ProcessEnv(manager, app.logger)

	// TODO move to functional options
	app.apmClient.ServerAPIKey = config.ApmServerApiKey
	app.apmClient.ServerSecretToken = config.ApmServerSecretToken

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

	if app.logsClient != nil {
		if err := app.logsClient.StartService([]logsapi.EventType{logsapi.Platform}, app.extensionClient.ExtensionID); err != nil {
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

	// The previous event id is used to validate the received Lambda metrics
	var prevEvent *extension.NextEventResponse
	// This data structure contains metadata tied to the current Lambda instance. If empty, it is populated once for each
	// active Lambda environment
	metadataContainer := apmproxy.MetadataContainer{}

	for {
		select {
		case <-ctx.Done():
			app.logger.Info("Received a signal, exiting...")

			return nil
		default:
			// Use a wait group to ensure the background go routine sending to the APM server
			// completes before signaling that the extension is ready for the next invocation.
			var backgroundDataSendWg sync.WaitGroup
			event, err := app.processEvent(ctx, &backgroundDataSendWg, prevEvent, &metadataContainer)
			if err != nil {
				return err
			}

			if event.EventType == extension.Shutdown {
				app.logger.Info("Received shutdown event, exiting...")
				return nil
			}
			app.logger.Debug("Waiting for background data send to end")
			backgroundDataSendWg.Wait()
			if config.SendStrategy == extension.SyncFlush {
				// Flush APM data now that the function invocation has completed
				app.apmClient.FlushAPMData(ctx)
			}
			prevEvent = event
		}
	}
}

func (app *App) processEvent(
	ctx context.Context,
	backgroundDataSendWg *sync.WaitGroup,
	prevEvent *extension.NextEventResponse,
	metadataContainer *apmproxy.MetadataContainer,
) (*extension.NextEventResponse, error) {

	// Invocation context
	invocationCtx, invocationCancel := context.WithCancel(ctx)
	defer invocationCancel()

	// call Next method of extension API.  This long polling HTTP method
	// will block until there's an invocation of the function
	app.logger.Infof("Waiting for next event...")
	event, err := app.extensionClient.NextEvent(ctx)
	if err != nil {
		app.logger.Errorf("Error: %s", err)

		status, errRuntime := app.extensionClient.ExitError(ctx, err.Error())
		if errRuntime != nil {
			return nil, errRuntime
		}

		app.logger.Infof("Exit signal sent to runtime : %s", status)
		app.logger.Infof("Exiting")
		return nil, err
	}

	// Used to compute Lambda Timeout
	event.Timestamp = time.Now()
	app.logger.Debug("Received event.")
	app.logger.Debugf("%v", extension.PrettyPrint(event))

	if event.EventType == extension.Shutdown {
		return event, nil
	}

	// APM Data Processing
	app.apmClient.AgentDoneSignal = make(chan struct{})
	defer close(app.apmClient.AgentDoneSignal)
	backgroundDataSendWg.Add(1)
	go func() {
		defer backgroundDataSendWg.Done()
		if err := app.apmClient.ForwardApmData(invocationCtx, metadataContainer); err != nil {
			app.logger.Error(err)
		}
	}()

	// Lambda Service Logs Processing, also used to extract metrics from APM logs
	// This goroutine should not be started if subscription failed
	runtimeDone := make(chan struct{})
	if app.logsClient != nil {
		go func() {
			if err := app.logsClient.ProcessLogs(invocationCtx, event.RequestID, app.apmClient, metadataContainer, runtimeDone, prevEvent); err != nil {
				app.logger.Errorf("Error while processing Lambda Logs ; %v", err)
			} else {
				close(runtimeDone)
			}
		}()
	} else {
		app.logger.Warn("Logs collection not started due to earlier subscription failure")
		close(runtimeDone)
	}

	// Calculate how long to wait for a runtimeDoneSignal or AgentDoneSignal signal
	flushDeadlineMs := event.DeadlineMs - 100
	durationUntilFlushDeadline := time.Until(time.Unix(flushDeadlineMs/1000, 0))

	// Create a timer that expires after durationUntilFlushDeadline
	timer := time.NewTimer(durationUntilFlushDeadline)
	defer timer.Stop()

	// The extension relies on 3 independent mechanisms to minimize the time interval between the end of the execution of
	// the lambda function and the end of the execution of processEvent()
	// 1) AgentDoneSignal is triggered upon reception of a `flushed=true` query from the agent
	// 2) [Backup 1] RuntimeDone is triggered upon reception of a Lambda log entry certifying the end of the execution of the current function
	// 3) [Backup 2] If all else fails, the extension relies of the timeout of the Lambda function to interrupt itself 100 ms before the specified deadline.
	// This time interval is large enough to attempt a last flush attempt (if SendStrategy == syncFlush) before the environment gets shut down.
	select {
	case <-app.apmClient.AgentDoneSignal:
		app.logger.Debug("Received agent done signal")
	case <-runtimeDone:
		app.logger.Debug("Received runtimeDone signal")
	case <-timer.C:
		app.logger.Info("Time expired waiting for agent signal or runtimeDone event")
	}

	return event, nil
}
