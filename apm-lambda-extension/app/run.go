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
	"elastic/apm-lambda-extension/apm"
	"elastic/apm-lambda-extension/extension"
	"elastic/apm-lambda-extension/logsapi"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// Run runs the app.
func (app *App) Run(ctx context.Context) error {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		extension.Log.Fatalf("failed to load default config: %v", err)
	}
	manager := secretsmanager.NewFromConfig(cfg)
	// pulls ELASTIC_ env variable into globals for easy access
	config := extension.ProcessEnv(manager)
	extension.Log.Level.SetLevel(config.LogLevel)

	// TODO move to functional options
	app.apmClient.ServerAPIKey = config.ApmServerApiKey
	app.apmClient.ServerSecretToken = config.ApmServerSecretToken

	// register extension with AWS Extension API
	res, err := app.extensionClient.Register(ctx, app.extensionName)
	if err != nil {
		extension.Log.Errorf("Error: %s", err)

		status, errRuntime := app.extensionClient.InitError(ctx, err.Error())
		if errRuntime != nil {
			return errRuntime
		}

		extension.Log.Infof("Init error signal sent to runtime : %s", status)
		extension.Log.Infof("Exiting")
		return err
	}
	extension.Log.Debugf("Register response: %v", extension.PrettyPrint(res))

	// start http server to receive data from agent
	err = app.apmClient.StartReceiver()
	if err != nil {
		extension.Log.Errorf("Could not start APM data receiver : %v", err)
	}
	defer func() {
		if err := app.apmClient.Shutdown(); err != nil {
			extension.Log.Warnf("Error while shutting down the apm receiver: %v", err)
		}
	}()

	if app.logsClient != nil {
		if err := app.logsClient.StartService([]logsapi.EventType{logsapi.Platform}, app.extensionClient.ExtensionID); err != nil {
			extension.Log.Warnf("Error while subscribing to the Logs API: %v", err)

			// disable logs API if the service failed to start
			app.logsClient = nil
		} else {
			// Remember to shutdown the log service if available.
			defer func() {
				if err := app.logsClient.Shutdown(); err != nil {
					extension.Log.Warnf("failed to shutdown the log service: %v", err)
				}
			}()
		}
	}

	// The previous event id is used to validate the received Lambda metrics
	var prevEvent *extension.NextEventResponse
	// This data structure contains metadata tied to the current Lambda instance. If empty, it is populated once for each
	// active Lambda environment
	metadataContainer := apm.MetadataContainer{}

	for {
		select {
		case <-ctx.Done():
			extension.Log.Info("Received a signal, exiting...")

			return nil
		default:
			// Use a wait group to ensure the background go routine sending to the APM server
			// completes before signaling that the extension is ready for the next invocation.
			var backgroundDataSendWg sync.WaitGroup
			event := app.processEvent(ctx, &backgroundDataSendWg, prevEvent, &metadataContainer)
			if event.EventType == extension.Shutdown {
				extension.Log.Info("Received shutdown event, exiting...")
				return nil
			}
			extension.Log.Debug("Waiting for background data send to end")
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
	metadataContainer *apm.MetadataContainer,
) *extension.NextEventResponse {

	// Invocation context
	invocationCtx, invocationCancel := context.WithCancel(ctx)
	defer invocationCancel()

	// call Next method of extension API.  This long polling HTTP method
	// will block until there's an invocation of the function
	extension.Log.Infof("Waiting for next event...")
	event, err := app.extensionClient.NextEvent(ctx)
	if err != nil {
		status, err := app.extensionClient.ExitError(ctx, err.Error())
		if err != nil {
			panic(err)
		}
		extension.Log.Errorf("Error: %s", err)
		extension.Log.Infof("Exit signal sent to runtime : %s", status)
		extension.Log.Infof("Exiting")
		return nil
	}

	// Used to compute Lambda Timeout
	event.Timestamp = time.Now()
	extension.Log.Debug("Received event.")
	extension.Log.Debugf("%v", extension.PrettyPrint(event))

	if event.EventType == extension.Shutdown {
		return event
	}

	// APM Data Processing
	app.apmClient.AgentDoneSignal = make(chan struct{})
	defer close(app.apmClient.AgentDoneSignal)
	backgroundDataSendWg.Add(1)
	go func() {
		defer backgroundDataSendWg.Done()
		if err := app.apmClient.ForwardApmData(invocationCtx, metadataContainer); err != nil {
			extension.Log.Error(err)
		}
	}()

	// Lambda Service Logs Processing, also used to extract metrics from APM logs
	// This goroutine should not be started if subscription failed
	runtimeDone := make(chan struct{})
	if app.logsClient != nil {
		go func() {
			if err := app.logsClient.ProcessLogs(invocationCtx, event.RequestID, app.apmClient, metadataContainer, runtimeDone, prevEvent); err != nil {
				extension.Log.Errorf("Error while processing Lambda Logs ; %v", err)
			} else {
				close(runtimeDone)
			}
		}()
	} else {
		extension.Log.Warn("Logs collection not started due to earlier subscription failure")
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
		extension.Log.Debug("Received agent done signal")
	case <-runtimeDone:
		extension.Log.Debug("Received runtimeDone signal")
	case <-timer.C:
		extension.Log.Info("Time expired waiting for agent signal or runtimeDone event")
	}

	return event
}
