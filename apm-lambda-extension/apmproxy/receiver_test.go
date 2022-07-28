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

package apmproxy_test

import (
	"bytes"
	"elastic/apm-lambda-extension/apmproxy"
	"elastic/apm-lambda-extension/logger"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInfoProxy(t *testing.T) {
	headers := map[string]string{"Authorization": "test-value"}
	wantResp := "{\"foo\": \"bar\"}"

	// Create apm server and handler
	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for key := range headers {
			assert.Equal(t, 1, len(r.Header[key]))
			assert.Equal(t, headers[key], r.Header[key][0])
		}
		w.Header().Add("test", "header")
		_, err := w.Write([]byte(`{"foo": "bar"}`))
		require.NoError(t, err)
	}))
	defer apmServer.Close()

	l, err := logger.New()
	require.NoError(t, err)

	// Create extension config and start the server
	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL(apmServer.URL),
		apmproxy.WithSecretToken("foo"),
		apmproxy.WithAPIKey("bar"),
		apmproxy.WithReceiverAddress(":1234"),
		apmproxy.WithReceiverTimeout(15*time.Second),
		apmproxy.WithLogger(l),
	)
	require.NoError(t, err)

	require.NoError(t, apmClient.StartReceiver())
	defer func() {
		require.NoError(t, apmClient.Shutdown())
	}()

	hosts, _ := net.LookupHost("localhost")
	url := "http://" + hosts[0] + ":1234"

	// Create a request to send to the extension
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	for name, value := range headers {
		req.Header.Add(name, value)
	}

	// Send the request to the extension
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, string(body), wantResp)
	assert.Equal(t, "header", resp.Header.Get("test"))
	require.NoError(t, resp.Body.Close())
}

func TestInfoProxyErrorStatusCode(t *testing.T) {
	// Create apm server and handler
	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer apmServer.Close()

	l, err := logger.New()
	require.NoError(t, err)

	// Create extension config and start the server
	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL(apmServer.URL),
		apmproxy.WithSecretToken("foo"),
		apmproxy.WithAPIKey("bar"),
		apmproxy.WithReceiverAddress(":1234"),
		apmproxy.WithReceiverTimeout(15*time.Second),
		apmproxy.WithLogger(l),
	)
	require.NoError(t, err)

	require.NoError(t, apmClient.StartReceiver())
	defer func() {
		require.NoError(t, apmClient.Shutdown())
	}()

	hosts, _ := net.LookupHost("localhost")
	url := "http://" + hosts[0] + ":1234"

	// Create a request to send to the extension
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)

	// Send the request to the extension
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func Test_handleInfoRequest(t *testing.T) {
	headers := map[string]string{"Authorization": "test-value"}
	// Copied from https://github.com/elastic/apm-server/blob/master/testdata/intake-v2/transactions.ndjson.
	agentRequestBody := `{"metadata": {"service": {"name": "1234_service-12a3","node": {"configured_name": "node-123"},"version": "5.1.3","environment": "staging","language": {"name": "ecmascript","version": "8"},"runtime": {"name": "node","version": "8.0.0"},"framework": {"name": "Express","version": "1.2.3"},"agent": {"name": "elastic-node","version": "3.14.0"}},"user": {"id": "123user", "username": "bar", "email": "bar@user.com"}, "labels": {"tag0": null, "tag1": "one", "tag2": 2}, "process": {"pid": 1234,"ppid": 6789,"title": "node","argv": ["node","server.js"]},"system": {"hostname": "prod1.example.com","architecture": "x64","platform": "darwin", "container": {"id": "container-id"}, "kubernetes": {"namespace": "namespace1", "pod": {"uid": "pod-uid", "name": "pod-name"}, "node": {"name": "node-name"}}},"cloud":{"account":{"id":"account_id","name":"account_name"},"availability_zone":"cloud_availability_zone","instance":{"id":"instance_id","name":"instance_name"},"machine":{"type":"machine_type"},"project":{"id":"project_id","name":"project_name"},"provider":"cloud_provider","region":"cloud_region","service":{"name":"lambda"}}}}
{"transaction": { "id": "945254c567a5417e", "trace_id": "0123456789abcdef0123456789abcdef", "parent_id": "abcdefabcdef01234567", "type": "request", "duration": 32.592981,  "span_count": { "started": 43 }}}
{"transaction": { "id": "00xxxxFFaaaa1234", "trace_id": "0123456789abcdef0123456789abcdef", "name": "amqp receive", "parent_id": "abcdefabcdef01234567", "type": "messaging", "duration": 3, "span_count": { "started": 1 }, "context": {"message": {"queue": { "name": "new_users"}, "age":{ "ms": 1577958057123}, "headers": {"user_id": "1ax3", "involved_services": ["user", "auth"]}, "body": "user created", "routing_key": "user-created-transaction"}},"session":{"id":"sunday","sequence":123}}}
`

	l, err := logger.New()
	require.NoError(t, err)

	// Create extension config
	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL("https://example.com"),
		apmproxy.WithSecretToken("foo"),
		apmproxy.WithAPIKey("bar"),
		apmproxy.WithReceiverAddress(":1234"),
		apmproxy.WithReceiverTimeout(15*time.Second),
		apmproxy.WithLogger(l),
	)
	require.NoError(t, err)

	// Start extension server
	require.NoError(t, apmClient.StartReceiver())
	defer func() {
		require.NoError(t, apmClient.Shutdown())
	}()

	// Create a request to send to the extension
	hosts, _ := net.LookupHost("localhost")
	url := "http://" + hosts[0] + ":1234/intake/v2/events"
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(agentRequestBody))
	require.NoError(t, err)
	// Add headers to the request
	for name, value := range headers {
		req.Header.Add(name, value)
	}

	// Send the request to the extension
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
}

func Test_handleIntakeV2EventsQueryParam(t *testing.T) {
	body := []byte(`{"metadata": {}`)

	// Create apm server and handler
	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer apmServer.Close()

	l, err := logger.New()
	require.NoError(t, err)

	// Create extension config and start the server
	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL(apmServer.URL),
		apmproxy.WithReceiverAddress(":1234"),
		apmproxy.WithReceiverTimeout(15*time.Second),
		apmproxy.WithLogger(l),
	)
	require.NoError(t, err)
	apmClient.AgentDoneSignal = make(chan struct{}, 1)
	require.NoError(t, apmClient.StartReceiver())
	defer func() {
		require.NoError(t, apmClient.Shutdown())
	}()

	hosts, _ := net.LookupHost("localhost")
	url := "http://" + hosts[0] + ":1234/intake/v2/events?flushed=true"

	// Create a request to send to the extension
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	require.NoError(t, err)

	// Send the request to the extension
	client := &http.Client{}
	go func() {
		_, err := client.Do(req)
		require.NoError(t, err)
	}()

	select {
	case <-apmClient.AgentDoneSignal:
		<-apmClient.DataChannel
	case <-time.After(1 * time.Second):
		t.Log("Timed out waiting for server to send FuncDone signal")
		t.Fail()
	}
}

func Test_handleIntakeV2EventsNoQueryParam(t *testing.T) {
	body := []byte(`{"metadata": {}`)

	// Create apm server and handler
	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer apmServer.Close()

	l, err := logger.New()
	require.NoError(t, err)

	// Create extension config and start the server
	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL(apmServer.URL),
		apmproxy.WithReceiverAddress(":1234"),
		apmproxy.WithReceiverTimeout(15*time.Second),
		apmproxy.WithLogger(l),
	)
	require.NoError(t, err)
	apmClient.AgentDoneSignal = make(chan struct{}, 1)
	require.NoError(t, apmClient.StartReceiver())
	defer func() {
		require.NoError(t, apmClient.Shutdown())
	}()

	hosts, _ := net.LookupHost("localhost")
	url := "http://" + hosts[0] + ":1234/intake/v2/events"

	// Create a request to send to the extension
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Logf("Could not create request")
	}

	// Send the request to the extension
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	select {
	case <-apmClient.DataChannel:
	case <-time.After(1 * time.Second):
		t.Log("Timed out waiting for server to send FuncDone signal")
		t.Fail()
	}
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
}

func Test_handleIntakeV2EventsQueryParamEmptyData(t *testing.T) {
	body := []byte(``)

	// Create apm server and handler
	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer apmServer.Close()

	l, err := logger.New()
	require.NoError(t, err)

	// Create extension config and start the server
	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL(apmServer.URL),
		apmproxy.WithReceiverAddress(":1234"),
		apmproxy.WithReceiverTimeout(15*time.Second),
		apmproxy.WithLogger(l),
	)
	require.NoError(t, err)
	apmClient.AgentDoneSignal = make(chan struct{}, 1)
	require.NoError(t, apmClient.StartReceiver())
	defer func() {
		require.NoError(t, apmClient.Shutdown())
	}()

	hosts, _ := net.LookupHost("localhost")
	url := "http://" + hosts[0] + ":1234/intake/v2/events?flushed=true"

	// Create a request to send to the extension
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	require.NoError(t, err)

	// Send the request to the extension
	client := &http.Client{}
	go func() {
		_, err := client.Do(req)
		require.NoError(t, err)
	}()

	select {
	case <-apmClient.AgentDoneSignal:
	case <-time.After(1 * time.Second):
		t.Log("Timed out waiting for server to send FuncDone signal")
		t.Fail()
	}
}
