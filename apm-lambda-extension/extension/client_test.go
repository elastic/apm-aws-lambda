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

package extension

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
)

func TestRegister(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	extensionName := "helloWorld"
	expectedRequest := `{"events":["INVOKE","SHUTDOWN"]}`
	response := []byte(`
		{
			"functionName": "helloWorld",
			"functionVersion": "$LATEST",
			"handler": "lambda_function.lambda_handler"
		}
	`)

	runtimeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bytes, _ := ioutil.ReadAll(r.Body)
		assert.Equal(t, expectedRequest, string(bytes))
		if _, err := w.Write(response); err != nil {
			t.Fail()
			return
		}
	}))
	defer runtimeServer.Close()

	client := NewClient(runtimeServer.Listener.Addr().String())
	res, err := client.Register(ctx, extensionName)
	require.NoError(t, err)
	assert.Equal(t, "helloWorld", res.FunctionName)
	assert.Equal(t, "$LATEST", res.FunctionVersion)
	assert.Equal(t, "lambda_function.lambda_handler", res.Handler)
}

func TestNextEvent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	response := []byte(`
		{
			"eventType": "INVOKE",
			"deadlineMs": 1646394703586,
			"requestId": "af4dbeb0-3761-451c-8b37-1c65cd02dde9",
			"invokedFunctionArn": "arn:aws:lambda:us-east-1:627286350134:function:Test",
			"tracing": {
				"type": "X-Amzn-Trace-Id",
				"value": "Root=1-6221fd44-5e7e917c1a0d50a7191543b5;Parent=561be8d807d7147c;Sampled=0"
			}
		}
	`)

	runtimeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write(response); err != nil {
			t.Fail()
			return
		}
	}))
	defer runtimeServer.Close()

	client := NewClient(runtimeServer.Listener.Addr().String())
	res, err := client.NextEvent(ctx)
	require.NoError(t, err)
	assert.Equal(t, Invoke, res.EventType)
	assert.Equal(t, int64(1646394703586), res.DeadlineMs)
	assert.Equal(t, "af4dbeb0-3761-451c-8b37-1c65cd02dde9", res.RequestID)
	assert.Equal(t, "arn:aws:lambda:us-east-1:627286350134:function:Test", res.InvokedFunctionArn)
	assert.Equal(t, "X-Amzn-Trace-Id", res.Tracing.Type)
	assert.Equal(t, "Root=1-6221fd44-5e7e917c1a0d50a7191543b5;Parent=561be8d807d7147c;Sampled=0", res.Tracing.Value)
}
