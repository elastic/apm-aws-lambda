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
	dataChannel := make(chan []byte)
	extension.NewHttpServer(dataChannel, config)

	// Make channel for collecting logs coming in as HTTP requests
	logsChannel := make(chan string)

	// Subscribe to the Logs API
	logsapi.Subscribe(extensionClient.ExtensionID, []logsapi.EventType{logsapi.Platform})

	// Create an http server to listen for log event requests
	logsApiListener, err := logsapi.NewLogsApiHttpListener(logsChannel)
	if err != nil {
		println("Error while creating Logs API listener: %v", err)
	}

	// Start the http server
	_, err = logsApiListener.Start()
	if err != nil {
		println("Error while starting Logs API listener: %v", err)
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

			// A shutdown event indicates the execution enviornment is shutting down.
			// This is usually due to inactivity.
			if res.EventType == extension.Shutdown {
				extension.ProcessShutdown()
				return
			}

			// // wait for platform.runtimeDone event
			// logsapi.waitForRuntimeDone()
			// // send to APM server
			// extension.ProcessAPMData(dataChannel, config)

			go func() {
				for {
					logs := <-logsChannel
					log.Printf("Received logs from Logs API: %v\n", logs)

				}
			}()

			extension.ProcessAPMData(dataChannel, config)
		}
	}
}
