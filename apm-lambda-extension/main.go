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
	"net/http"
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
	extension.Log.Logger.SetLevel(config.LogLevel)

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

	// Create a channel to buffer apm agent data
	agentDataChannel := make(chan extension.AgentData, 100)

	// Start http server to receive data from agent
	if err = extension.StartHttpServer(agentDataChannel, config); err != nil {
		extension.Log.Errorf("Could not start APM data receiver : %v", err)
	}

	// Create a client to use for sending data to the apm server
	client := &http.Client{
		Timeout:   time.Duration(config.DataForwarderTimeoutSeconds) * time.Second,
		Transport: http.DefaultTransport.(*http.Transport).Clone(),
	}

	// Create a channel used

	// Make channel for collecting logs and create a HTTP server to listen for them
	logsChannel := make(chan logsapi.LogEvent)

	// Use a wait group to ensure the background go routine sending to the APM server
	// completes before signaling that the extension is ready for the next invocation.
	var backgroundDataSendWg sync.WaitGroup

	if err := logsapi.Subscribe(
		ctx,
		extensionClient.ExtensionID,
		[]logsapi.EventType{logsapi.Platform},
		logsChannel,
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

			// Make a channel for signaling that we received the agent flushed signal
			extension.AgentDoneSignal = make(chan struct{})
			// Make a channel for signaling that we received the runtimeDone logs API event
			runtimeDoneSignal := make(chan struct{})
			// Make a channel for signaling that the function invocation is complete
			funcDone := make(chan struct{})

			// A shutdown event indicates the execution environment is shutting down.
			// This is usually due to inactivity.
			if event.EventType == extension.Shutdown {
				extension.ProcessShutdown()
				cancel()
				return
			}

			// Receive agent data as it comes in and post it to the APM server.
			// Stop checking for, and sending agent data when the function invocation
			// has completed, signaled via a channel.
			backgroundDataSendWg.Add(1)
			go func() {
				defer backgroundDataSendWg.Done()
				if !extension.IsTransportStatusHealthyOrPending() {
					return
				}
				for {
					select {
					case <-funcDone:
						extension.Log.Debug("Received signal that function has completed, not processing any more agent data")
						return
					case agentData := <-agentDataChannel:
						if err := extension.PostToApmServer(client, agentData, config, ctx); err != nil {
							extension.Log.Errorf("Error sending to APM server, skipping: %v", err)
							extension.EnqueueAPMData(agentDataChannel, agentData)
							return
						}
					}
				}
			}()

			// Receive Logs API events
			// Send to the runtimeDoneSignal channel to signal when a runtimeDone event is received
			go func() {
				for {
					select {
					case <-funcDone:
						extension.Log.Debug("Received signal that function has completed, not processing any more log events")
						return
					case logEvent := <-logsChannel:
						extension.Log.Debugf("Received log event %v", logEvent.Type)
						// Check the logEvent for runtimeDone and compare the RequestID
						// to the id that came in via the Next API
						if logEvent.Type == logsapi.RuntimeDone {
							if logEvent.Record.RequestId == event.RequestID {
								extension.Log.Info("Received runtimeDone event for this function invocation")
								runtimeDoneSignal <- struct{}{}
								return
							} else {
								extension.Log.Debug("Log API runtimeDone event request id didn't match")
							}
						}
					}
				}
			}()

			// Calculate how long to wait for a runtimeDoneSignal or AgentDoneSignal signal
			flushDeadlineMs := event.DeadlineMs - 100
			durationUntilFlushDeadline := time.Until(time.Unix(flushDeadlineMs/1000, 0))

			// Create a timer that expires after durationUntilFlushDeadline
			timer := time.NewTimer(durationUntilFlushDeadline)
			defer timer.Stop()

			select {
			case <-extension.AgentDoneSignal:
				extension.Log.Debug("Received agent done signal")
			case <-runtimeDoneSignal:
				extension.Log.Debug("Received runtimeDone signal")
			case <-timer.C:
				extension.Log.Info("Time expired waiting for agent signal or runtimeDone event")
			}

			close(funcDone)
			backgroundDataSendWg.Wait()
			if config.SendStrategy == extension.SyncFlush {
				// Flush APM data now that the function invocation has completed
				extension.FlushAPMData(client, agentDataChannel, config, ctx)
			}
		}
	}
}
