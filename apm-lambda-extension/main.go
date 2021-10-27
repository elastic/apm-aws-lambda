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

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
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

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		s := <-sigs
		cancel()
		log.Printf("Received %v\n", s)
		log.Println("Exiting")
	}()

	// register extension with AWS Extension API
	res, err := extensionClient.Register(ctx, extensionName)
	if err != nil {
		panic(err)
	}
	log.Printf("Register response: %v\n", extension.PrettyPrint(res))

	// pulls ELASTIC_ env variable into globals for easy access
	config := extension.ProcessEnv()

	// setup http server to receive data from agent
	// and get a channel to listen for that data
	dataChannel := make(chan []byte, 100)

	extension.NewHttpServer(dataChannel, config)

	// Make channel for collecting logs and create a HTTP server to listen for them
	logsChannel := make(chan logsapi.LogEvent)

	// Subscribe to the Logs API
	err = logsapi.Subscribe(
		extensionClient.ExtensionID,
		[]logsapi.EventType{logsapi.Platform})
	if err != nil {
		log.Printf("Could not subscribe to the logs API. Will instead flush APM data 100ms before the function deadline.")
	} else {
		logsAPIListener, err := logsapi.NewLogsAPIHttpListener(logsChannel)
		if err != nil {
			log.Printf("Error while creating Logs API listener: %v", err)
		}

		// Start the logs HTTP server
		_, err = logsAPIListener.Start(logsapi.ListenOnAddress())
		if err != nil {
			log.Printf("Error while starting Logs API listener: %v", err)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// call Next method of extension API.  This long polling HTTP method
			// will block until there's an invocation of the function
			log.Println("Waiting for next event...")
			event, err := extensionClient.NextEvent(ctx)
			if err != nil {
				log.Printf("Error: %v\n", err)
				log.Println("Exiting")
				return
			}
			log.Printf("Received event: %v\n", extension.PrettyPrint(event))

			// A shutdown event indicates the execution environment is shutting down.
			// This is usually due to inactivity.
			if event.EventType == extension.Shutdown {
				extension.ProcessShutdown()
				return
			}

			// Flush any APM data, in case waiting for the runtimeDone event timed out,
			// the agent data wasn't available yet, and we got to the next event
			extension.FlushAPMData(dataChannel, config)

			// Make a channel for signaling that a runtimeDone event has been received
			runtimeDone := make(chan struct{})

			// Make a channel for signaling that that function invocation has completed
			funcInvocDone := make(chan struct{})

			// Receive agent data as it comes in and post it to the APM server.
			// Stop checking for, and sending agent data when the function invocation
			// has completed, signaled via a channel.
			go func() {
				for {
					select {
					case <-funcInvocDone:
						log.Println("Function invocation is complete, not receiving any more agent data")
						return
					case agentData := <-dataChannel:
						err := extension.PostToApmServer(agentData, config)
						if err != nil {
							log.Printf("Error sending to APM server, skipping: %v", err)
						}
					}
				}
			}()

			// Receive Logs API events
			// Send to the runtimeDone channel to signal when a runtimeDone event is received
			go func() {
				for {
					select {
					case <-funcInvocDone:
						log.Println("Function invocation is complete, not receiving any more log events")
						return
					case logEvent := <-logsChannel:
						log.Printf("Received log event %v\n", logEvent.Type)
						// Check the logEvent for runtimeDone and compare the RequestID
						// to the id that came in via the Next API
						if logsapi.SubEventType(logEvent.Type) == logsapi.RuntimeDone {
							if logEvent.Record.RequestId == event.RequestID {
								log.Printf("Received runtimeDone event %v", logEvent)
								runtimeDone <- struct{}{}
								return
							} else {
								log.Println("Log API runtimeDone event request id didn't match")
							}
						}
					}
				}
			}()

			// Calculate how long to wait for a runtimeDone event
			funcTimeout := time.Unix(0, event.DeadlineMs*int64(time.Millisecond))
			msBeforeFuncTimeout := 100 * time.Millisecond
			timeToWait := funcTimeout.Sub(time.Now()) - msBeforeFuncTimeout

			// Create a timer that expires after timeToWait
			timer := time.NewTimer(timeToWait)
			defer timer.Stop()

			select {
			case <-runtimeDone:
				log.Println("Received runtimeDone event signal")
			case <-timer.C:
				log.Println("Time expired waiting for runtimeDone event")
			}

			// Flush APM data now that the function invocation has completed
			extension.FlushAPMData(dataChannel, config)

			// Signal that the function invocation has completed
			close(funcInvocDone)
		}
	}
}
