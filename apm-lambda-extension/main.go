// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT-0

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

	// Subscribe to the Logs API
	logsapi.Subscribe(extensionClient.ExtensionID, []logsapi.EventType{logsapi.Platform})

	// Make channel for collecting logs and create a HTTP server to listen for them
	logsChannel := make(chan logsapi.LogEvent)
	logsApiListener, err := logsapi.NewLogsApiHttpListener(logsChannel)
	if err != nil {
		log.Printf("Error while creating Logs API listener: %v", err)
	}

	// Start the logs HTTP server
	_, err = logsApiListener.Start()
	if err != nil {
		log.Printf("Error while starting Logs API listener: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// call Next method of extension API.  This long polling HTTP method
			// will block until there's an invocation of the function
			log.Println("Waiting for event...")
			res, err := extensionClient.NextEvent(ctx)
			if err != nil {
				log.Printf("Error: %v\n", err)
				log.Println("Exiting")
				return
			}
			log.Printf("Received event: %v\n", extension.PrettyPrint(res))

			// Flush any APM data, in case waiting for the runtimeDone event timed out,
			// the agent data wasn't available yet, and we got to the next event
			extension.FlushAPMData(dataChannel, config)

			// A shutdown event indicates the execution environment is shutting down.
			// This is usually due to inactivity.
			if res.EventType == extension.Shutdown {
				extension.ProcessShutdown()
				return
			}

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
						return
					case agentData := <-dataChannel:
						extension.PostToApmServer(agentData, config)
					}
				}
			}()

			// Receive Logs API events
			// Send to the runtimeDone channel to signal when a runtimeDone event is received
			go func() {
				for {
					select {
					case <-funcInvocDone:
						return
					case logEvent := <-logsChannel:
						log.Printf("Received log event %v\n", logEvent)
						// Check the logEvent for runtimeDone and compare the RequestID
						// to the id that came in via the Next API
						if logsapi.SubEventType(logEvent.Type) == logsapi.RuntimeDone {
							if logEvent.Record.RequestId == res.RequestID {
								runtimeDone <- struct{}{}
								return
							}
						}
					}
				}
			}()

			// Calculate how long to wait for a runtimeDone event
			funcTimeout := time.Unix(0, res.DeadlineMs*int64(time.Millisecond))
			msBeforeFuncTimeout := 100 * time.Millisecond
			timeToWait := funcTimeout.Sub(time.Now()) - msBeforeFuncTimeout

			// Create a timer that expires after timeToWait
			timer := time.NewTimer(timeToWait)
			defer timer.Stop()

			select {
			case <-runtimeDone:
				log.Println("Received runtimeDone event.")
			case <-timer.C:
				log.Println("Time expired waiting for runtimeDone event.")
			}

			// Flush APM data now that the function invocation has completed
			extension.FlushAPMData(dataChannel, config)

			// Signal that the function invocation has completed
			funcInvocDone <- struct{}{}
		}
	}
}
