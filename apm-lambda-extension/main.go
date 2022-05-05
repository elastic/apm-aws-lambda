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

package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"elastic/apm-lambda-extension/extension"
	"elastic/apm-lambda-extension/logsapi"
)

var (
	extensionName   = filepath.Base(os.Args[0]) // extension name has to match the filename
	extensionClient = extension.NewClient(os.Getenv("AWS_LAMBDA_RUNTIME_API"))
)

/* --- elastic vars  --- */

func main() {

	ctx, cancel := context.WithCancel(context.Background())

	// Trigger ctx.Done() in all relevant goroutines when main ends
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		s := <-sigs
		cancel()
		extension.Log.Infof("Received %v\n, exiting", s)
	}()

	// pulls ELASTIC_ env variable into globals for easy access
	config := extension.ProcessEnv()
	extension.Log.Level.SetLevel(config.LogLevel)

	// register extension with AWS Extension API
	res, err := extensionClient.Register(ctx, extensionName)
	if err != nil {
		status, errRuntime := extensionClient.InitError(ctx, err.Error())
		if errRuntime != nil {
			panic(errRuntime)
		}
		extension.Log.Errorf("Error: %s", err)
		extension.Log.Infof("Init error signal sent to runtime : %s", status)
		extension.Log.Infof("Exiting")
		return
	}
	extension.Log.Debugf("Register response: %v", extension.PrettyPrint(res))

	// Init APM Server Transport struct
	apmServerTransport := extension.InitApmServerTransport(config)

	// Start http server to receive data from agent
	agentDataServer, err := extension.StartHttpServer(ctx, apmServerTransport)
	if err != nil {
		extension.Log.Errorf("Could not start APM data receiver : %v", err)
	}

	// Init APM Server Transport struct
	// Make channel for collecting logs and create a HTTP server to listen for them
	logsTransport := logsapi.InitLogsTransport()

	// Use a wait group to ensure the background go routine sending to the APM server
	// completes before signaling that the extension is ready for the next invocation.
	var backgroundDataSendWg sync.WaitGroup

	if err := logsapi.Subscribe(
		ctx,
		logsTransport,
		extensionClient.ExtensionID,
		[]logsapi.EventType{logsapi.Platform},
	); err != nil {
		extension.Log.Warnf("Error while subscribing to the Logs API: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// call Next method of extension API.  This long polling HTTP method
			// will block until there's an invocation of the function
			extension.Log.Infof("Waiting for next event...")
			event, err := extensionClient.NextEvent(ctx)
			if err != nil {
				status, err := extensionClient.ExitError(ctx, err.Error())
				if err != nil {
					panic(err)
				}
				extension.Log.Errorf("Error: %s", err)
				extension.Log.Infof("Exit signal sent to runtime : %s", status)
				extension.Log.Infof("Exiting")
				return
			}
			extension.Log.Debug("Received event.")
			extension.Log.Debugf("%v", extension.PrettyPrint(event))

			// Make a channel for signaling that the function invocation is complete
			funcDone := make(chan struct{})

			// A shutdown event indicates the execution environment is shutting down.
			// This is usually due to inactivity.
			if event.EventType == extension.Shutdown {
				extension.ProcessShutdown(agentDataServer, logsTransport.Server)
				cancel()
				return
			}

			backgroundDataSendWg.Add(1)
			extension.StartBackgroundApmDataForwarding(ctx, apmServerTransport, funcDone, &backgroundDataSendWg)
			logsapi.StartBackgroundLogsProcessing(logsTransport, funcDone, event.RequestID)

			// Calculate how long to wait for a runtimeDoneSignal or AgentDoneSignal signal
			flushDeadlineMs := event.DeadlineMs - 100
			durationUntilFlushDeadline := time.Until(time.Unix(flushDeadlineMs/1000, 0))

			// Create a timer that expires after durationUntilFlushDeadline
			timer := time.NewTimer(durationUntilFlushDeadline)
			defer timer.Stop()

			select {
			case <-apmServerTransport.AgentDoneSignal:
				extension.Log.Debug("Received agent done signal")
			case <-logsTransport.RuntimeDoneSignal:
				extension.Log.Debug("Received runtimeDone signal")
			case <-timer.C:
				extension.Log.Info("Time expired waiting for agent signal or runtimeDone event")
			}

			close(funcDone)
			backgroundDataSendWg.Wait()
			if config.SendStrategy == extension.SyncFlush {
				// Flush APM data now that the function invocation has completed
				extension.FlushAPMData(ctx, apmServerTransport)
			}
		}
	}
}
