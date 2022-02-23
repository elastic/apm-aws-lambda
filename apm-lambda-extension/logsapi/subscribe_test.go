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

package logsapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubscribeWithSamLocalEnv(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	os.Setenv("AWS_SAM_LOCAL", "true")
	t.Cleanup(func() {
		os.Unsetenv("AWS_SAM_LOCAL")
	})
	out := make(chan LogEvent)

	err := Subscribe(ctx, "testing123", []EventType{Platform}, out)
	assert.Error(t, err)
}

func TestSubscribeAwsRequest(t *testing.T) {
	listenerAddress = "localhost:0"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan LogEvent)
	// For subscription request
	expectedTypes := []EventType{Platform}
	expectedBufferingCfg := BufferingCfg{
		MaxItems:  10000,
		MaxBytes:  262144,
		TimeoutMS: 25,
	}
	// For logs API event
	platformDoneEvent := `{
		"time": "2021-02-04T20:00:05.123Z",
		"type": "platform.runtimeDone",
		"record": {
		   "requestId":"6f7f0961f83442118a7af6fe80b88",
		   "status": "success"
		}
	}`
	body := []byte(`[` + platformDoneEvent + `]`)

	// Create aws runtime API server and handler
	awsRuntimeApiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request_bytes, _ := ioutil.ReadAll(r.Body)
		req := SubscribeRequest{}
		json.Unmarshal(request_bytes, &req)
		// Validate the subscription request
		assert.True(t, strings.Contains(string(req.Destination.URI), "sandbox"))
		assert.Equal(t, req.BufferingCfg, expectedBufferingCfg)
		assert.Equal(t, req.EventTypes, expectedTypes)
	}))
	defer awsRuntimeApiServer.Close()

	// Set the Runtime server address as an env variable
	os.Setenv("AWS_LAMBDA_RUNTIME_API", awsRuntimeApiServer.Listener.Addr().String())

	// Subscribe to the logs api and start the http server listening for events
	err := Subscribe(ctx, "testing123", []EventType{Platform}, out)
	if err != nil {
		t.Logf("Error subscribing, %v", err)
		t.Fail()
		return
	}
	defer logsAPIServer.Close()

	// Create a request to send to the logs listener
	url := "http://" + logsAPIListener.Addr().String()
	req, err := http.NewRequest("GET", url, bytes.NewReader(body))
	if err != nil {
		t.Log("Could not create request")
	}

	// Send the request to the logs listener
	client := &http.Client{}
	go func() {
		_, err = client.Do(req)
	}()

	if err != nil {
		t.Logf("Error fetching %s, [%v]", url, err)
		t.Fail()
	} else {
		event := <-out
		assert.Equal(t, event.Record.RequestId, "6f7f0961f83442118a7af6fe80b88")
	}
}
