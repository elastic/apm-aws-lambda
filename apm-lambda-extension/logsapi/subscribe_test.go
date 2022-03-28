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
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscribeWithSamLocalEnv(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := os.Setenv("AWS_SAM_LOCAL", "true"); err != nil {
		t.Fail()
	}
	t.Cleanup(func() {
		if err := os.Unsetenv("AWS_SAM_LOCAL"); err != nil {
			t.Fail()
		}
	})
	out := make(chan LogEvent)

	err := Subscribe(ctx, "testID", []EventType{Platform}, out)
	assert.Error(t, err)
}

func TestSubscribeAWSRequest(t *testing.T) {
	ListenerHost = "localhost"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan LogEvent, 1)
	// For subscription request
	expectedTypes := []EventType{Platform}
	expectedBufferingCfg := BufferingCfg{
		MaxItems:  10000,
		MaxBytes:  262144,
		TimeoutMS: 25,
	}

	// Create aws runtime API server and handler
	awsRuntimeApiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := SubscribeRequest{}
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		// Validate the subscription request
		assert.Equal(t, req.BufferingCfg, expectedBufferingCfg)
		assert.Equal(t, req.EventTypes, expectedTypes)
	}))
	defer awsRuntimeApiServer.Close()

	// Set the Runtime server address as an env variable
	if err := os.Setenv("AWS_LAMBDA_RUNTIME_API", awsRuntimeApiServer.Listener.Addr().String()); err != nil {
		return
	}

	// Subscribe to the logs api and start the http server listening for events
	if err := Subscribe(ctx, "testID", []EventType{Platform}, out); err != nil {
		t.Logf("Error subscribing, %v", err)
		t.Fail()
		return
	}
	defer Server.Close()

	// Create a request to send to the logs listener
	platformDoneEvent := `{
		"time": "2021-02-04T20:00:05.123Z",
		"type": "platform.runtimeDone",
		"record": {
		   "requestId":"6f7f0961f83442118a7af6fe80b88",
		   "status": "success"
		}
	}`
	body := []byte(`[` + platformDoneEvent + `]`)
	url := "http://" + Listener.Addr().String()
	req, err := http.NewRequest("GET", url, bytes.NewReader(body))
	if err != nil {
		t.Log("Could not create request")
	}

	// Send the request to the logs listener
	client := http.DefaultClient
	if _, err = client.Do(req); err != nil {
		t.Logf("Error fetching %s, [%v]", url, err)
		t.Fail()
	}
	event := <-out
	assert.Equal(t, event.Record.RequestId, "6f7f0961f83442118a7af6fe80b88")
}

func TestSubscribeWithBadLogsRequest(t *testing.T) {
	ListenerHost = "localhost"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan LogEvent)

	// Create aws runtime API server and handler
	awsRuntimeApiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer awsRuntimeApiServer.Close()

	// Set the Runtime server address as an env variable
	if err := os.Setenv("AWS_LAMBDA_RUNTIME_API", awsRuntimeApiServer.Listener.Addr().String()); err != nil {
		t.Fail()
		return
	}

	// Subscribe to the logs api and start the http server listening for events
	if err := Subscribe(ctx, "testID", []EventType{Platform}, out); err != nil {
		t.Logf("Error subscribing, %v", err)
		t.Fail()
		return
	}
	defer Server.Close()

	// Create a request to send to the logs listener
	logEvent := `{"invalid": "json"}`
	body := []byte(`[` + logEvent + `]`)
	url := "http://" + Listener.Addr().String()
	req, err := http.NewRequest("GET", url, bytes.NewReader(body))
	if err != nil {
		t.Log("Could not create request")
	}

	// Send the request to the logs listener
	client := http.DefaultClient
	resp, err := client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, resp.StatusCode, 500)
}
