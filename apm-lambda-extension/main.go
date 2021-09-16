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
	dataChannel := make(chan []byte, 1)
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

			// Check if there's APM data, in case waiting for the runtimeDone event timed out,
			// the agent data wasn't available yet and we got to the next event
			extension.FlushAPMData(dataChannel, config)

			// A shutdown event indicates the execution enviornment is shutting down.
			// This is usually due to inactivity.
			if res.EventType == extension.Shutdown {
				extension.ProcessShutdown()
				return
			}

			// Receive Logs API events
			// Send to the runtimeDone channel to signal when a runtimeDone event is received
			// Todo: do we need to close the channel after the runtimeDone event is received?
			runtimeDone := make(chan bool)
			go func() {
				for logEvent := range logsChannel {
					log.Printf("Received log event %v\n", logEvent)
					//check the logEvent for runtimeDone
					if logsapi.SubEventType(logEvent.Type) == logsapi.RuntimeDone {
						runtimeDone <- true
						break
					}
				}
			}()

			// Calculate how long to wait for a runtimeDone event
			funcTimeout := time.Unix(0, res.DeadlineMs*int64(time.Millisecond))
			msBeforeFuncTimeout := 100 * time.Millisecond
			timeToWait := funcTimeout.Sub(time.Now()) - msBeforeFuncTimeout

			select {
			case <-runtimeDone:
				log.Println("Received runtimeDone event, flushing APM data")
				extension.FlushAPMData(dataChannel, config)
			case <-time.After(timeToWait):
				log.Println("Time expired waiting for runtimeDone event. Attempting to read agent data.")
				extension.FlushAPMData(dataChannel, config)
			}
		}
	}
}
