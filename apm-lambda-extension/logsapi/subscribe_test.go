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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubscribeWithSamLocalTest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	os.Setenv("AWS_SAM_LOCAL", "true")

	out := make(chan LogEvent)

	Subscribe(ctx, "testing123", []EventType{Platform}, out)
	logsAPIServer.Close()
}

func TestSubscribeWithListenerEnvVariable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	os.Setenv("ELASTIC_APM_LAMBDA_LOGS_LISTENER_ADDRESS", "localhost:5678")
	defer t.Cleanup(func() {
		os.Unsetenv("ELASTIC_APM_LAMBDA_LOGS_LISTENER_ADDRESS")
	})
	platformDoneEvent := `{
		"time": "2021-02-04T20:00:05.123Z",
		"type": "platform.runtimeDone",
		"record": {
		   "requestId":"6f7f0961f83442118a7af6fe80b88",
		   "status": "success"
		}
	}`
	body := []byte(`[` + platformDoneEvent + `]`)
	out := make(chan LogEvent)

	Subscribe(ctx, "testing123", []EventType{Platform}, out)
	defer logsAPIServer.Close()

	// Create a request to send to the extension
	url := "http://" + "localhost" + ":5678"
	req, err := http.NewRequest("GET", url, bytes.NewReader(body))
	if err != nil {
		t.Logf("Could not create request")
	}

	// Send the request to the logs API
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

func TestSubscribeWithListenerRandomPort(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	platformDoneEvent := `{
		"time": "2021-02-04T20:00:05.123Z",
		"type": "platform.runtimeDone",
		"record": {
		   "requestId":"6f7f0961f83442118a7af6fe80b88",
		   "status": "success"
		}
	}`
	body := []byte(`[` + platformDoneEvent + `]`)
	out := make(chan LogEvent)

	Subscribe(ctx, "testing123", []EventType{Platform}, out)
	defer logsAPIServer.Close()

	// Create a request to send to the extension
	url := "http://" + logsAPIListener.Addr().String()
	req, err := http.NewRequest("GET", url, bytes.NewReader(body))
	if err != nil {
		t.Logf("Could not create request")
	}

	// Send the request to the logs API
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

func TestSubscribeRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan LogEvent)
	expectedTypes := []EventType{Platform}
	expectedBufferingCfg := BufferingCfg{
		MaxItems:  10000,
		MaxBytes:  262144,
		TimeoutMS: 25,
	}

	// Create aws runtime API server and handler
	awsRuntimeApiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request_bytes, _ := ioutil.ReadAll(r.Body)
		req := SubscribeRequest{}
		json.Unmarshal(request_bytes, &req)
		assert.Equal(t, req.BufferingCfg, expectedBufferingCfg)
		assert.Equal(t, req.EventTypes, expectedTypes)
	}))
	defer awsRuntimeApiServer.Close()

	os.Setenv("AWS_LAMBDA_RUNTIME_API", awsRuntimeApiServer.Listener.Addr().String())
	Subscribe(ctx, "testing123", []EventType{Platform}, out)
	logsAPIServer.Close()
}
