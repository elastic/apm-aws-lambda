// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT-0

package main

import "log"
import "bytes"
import "compress/gzip"
import "net/http"
import "io"
import "io/ioutil"


import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"elastic/apm-lambda-extension/extension"
)

var (
	extensionName   = filepath.Base(os.Args[0]) // extension name has to match the filename
	extensionClient = extension.NewClient(os.Getenv("AWS_LAMBDA_RUNTIME_API"))
	printPrefix     = fmt.Sprintf("[%s]", extensionName)
)

/* --- elastic vars  --- */
var (
	endpoint string
	token string
	socketPath = "/tmp/elastic-apm-data"
)
/* --- elastic vars  --- */

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		s := <-sigs
		cancel()
		println(printPrefix, "Received", s)
		println(printPrefix, "Exiting")
	}()

	// register extension with AWS Extension API
	res, err := extensionClient.Register(ctx, extensionName)
	if err != nil {
		panic(err)
	}
	println(printPrefix, "Register response:", prettyPrint(res))

	// pulls ELASTIC_ env variable into globals for easy access
	processEnv()
	// setup named/fifo pipe for data and get a channel into that pipe
	// dataChannel := setupNamedSocket()
	dataChannel := extension.NewHttpServer()
	// Will block until shutdown event is received or cancelled via the context.
	processEvents(ctx, dataChannel)
}

func processEvents(ctx context.Context, dataChannel chan []byte) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// call Next method of extension API.  This long polling HTTP method
			// will block until there's an invocation of the function
			println(printPrefix, "Waiting for event...")
			res, err := extensionClient.NextEvent(ctx)
			if err != nil {
				println(printPrefix, "Error:", err)
				println(printPrefix, "Exiting")
				return
			}
			println(printPrefix, "Received event:", prettyPrint(res))

			// A shutdown event indicates the execution enviornment is shutting down.
			// This is usually due to inactivity.
			if res.EventType == extension.Shutdown {
				println(printPrefix, "Received SHUTDOWN event")
				println(printPrefix, "Exiting")
				return
			}

			// Wait for agent to send data to the channel
			//
			// to do: this will hang if the lambda times out or crashes or the agent
			//        fails to send data to the pipe
			select {
				case agentBytes := <-dataChannel:
					println(printPrefix, "received bytes, will try to post.  Here they are as a string", string(agentBytes))
					postToApmServer(agentBytes)
					println(printPrefix, "done with post")
			}

		}
	}
}

func prettyPrint(v interface{}) string {
	data, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return ""
	}
	return string(data)
}

// pull env into globals
func processEnv() {
	println(printPrefix, "sending to " + os.Getenv("ELASTIC_APM_SERVER_URL") + "\n\n")

	endpoint = os.Getenv("ELASTIC_APM_SERVER_URL") + "/intake/v2/events"
	token = os.Getenv("ELASTIC_APM_SECRET_TOKEN")
	if("" == endpoint) {
		log.Fatalln("please set ELASTIC_APM_SERVER_URL, exiting")
	}
	if("" == token) {
		log.Fatalln("please set ELASTIC_APM_SECRET_TOKEN, exiting")
	}
}

func setupNamedSocket() chan []byte{
	err := syscall.Mkfifo(socketPath, 0666)
	if(nil != err) {
		println(printPrefix, "error creating pipe", err)
	}
	dataChannel := make(chan []byte)
	go func() {
		for {
			dataChannel <- pollSocketForData()
		}
	}()
	return dataChannel
}

// todo: can this be a streaming or streaming style call that keeps the
//       connection open across invocations?
func postToApmServer(postBody []byte) {
	var compressedBytes bytes.Buffer
	w := gzip.NewWriter(&compressedBytes)
	w.Write(postBody)
	w.Write([]byte{10})
	w.Close()

	client := &http.Client{}

 	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(compressedBytes.Bytes()))
	if err != nil {
		log.Fatalf("An Error Occured calling NewRequest %v", err)
 	}
	req.Header.Add("Content-Type","application/x-ndjson")
	req.Header.Add("Content-Encoding","gzip")
	req.Header.Add("Authorization", "Bearer " + token)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("An Error Occured calling client.Do %v", err)
  }

	//Read the response body
	defer resp.Body.Close()
	println(printPrefix, "reading response body")
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		println(printPrefix, "there was an error reading the resp body")
		log.Fatalln(err)
	}
	println(printPrefix, "Response: ")
	sb := string(body)
	println(printPrefix, sb)
}

func pollSocketForData() []byte {
	dataPipe, err := os.OpenFile(socketPath, os.O_RDONLY, 0)
	if err != nil {
		log.Fatalln("failed to open pipe", err)
	}

	defer close(dataPipe)

	// When the write side closes, we get an EOF.
	bytes, err := ioutil.ReadAll(dataPipe)
	if err != nil {
		log.Fatalln("failed to read telemetry pipe", err)
	}

	return bytes
}

func close(thing io.Closer) {
	err := thing.Close()
	if err != nil {
		log.Println(err)
	}
}
