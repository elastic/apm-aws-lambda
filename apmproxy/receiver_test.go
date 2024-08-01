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
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/elastic/apm-aws-lambda/apmproxy"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestInfoProxy(t *testing.T) {
	headers := map[string]string{"Authorization": "test-value"}
	wantResp := "{\"foo\": \"bar\"}"

	// Create apm server and handler
	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for key := range headers {
			assert.Len(t, r.Header[key], 2)
			assert.Equal(t, headers[key], r.Header[key][0])
		}
		w.Header().Add("test", "header")
		_, err := w.Write([]byte(`{"foo": "bar"}`))
		assert.NoError(t, err)
	}))
	defer apmServer.Close()

	// Create extension config and start the server
	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL(apmServer.URL),
		apmproxy.WithSecretToken("foo"),
		apmproxy.WithAPIKey("bar"),
		// Use ipv4 to avoid issues in CI
		apmproxy.WithReceiverAddress("127.0.0.1:1234"),
		apmproxy.WithReceiverTimeout(15*time.Second),
		apmproxy.WithLogger(zaptest.NewLogger(t).Sugar()),
	)
	require.NoError(t, err)

	require.NoError(t, apmClient.StartReceiver())
	defer func() {
		require.NoError(t, apmClient.Shutdown())
	}()

	url := "http://127.0.0.1:1234"

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
	assert.Equal(t, wantResp, string(body))
	assert.Equal(t, "header", resp.Header.Get("test"))
	require.NoError(t, resp.Body.Close())
}

func TestInfoProxyAuth(t *testing.T) {
	// Create apm server and handler
	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "ApiKey bar", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusTeapot)
	}))
	defer apmServer.Close()

	// Create extension config and start the server
	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL(apmServer.URL),
		apmproxy.WithAPIKey("bar"),
		// Use ipv4 to avoid issues in CI
		apmproxy.WithReceiverAddress("127.0.0.1:1234"),
		apmproxy.WithReceiverTimeout(15*time.Second),
		apmproxy.WithLogger(zaptest.NewLogger(t).Sugar()),
	)
	require.NoError(t, err)

	require.NoError(t, apmClient.StartReceiver())
	defer func() {
		require.NoError(t, apmClient.Shutdown())
	}()

	url := "http://127.0.0.1:1234"

	// Send the request to the extension
	resp, err := http.Get(url)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
}

func TestInfoProxyErrorStatusCode(t *testing.T) {
	// Create apm server and handler
	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer apmServer.Close()

	// Create extension config and start the server
	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL(apmServer.URL),
		apmproxy.WithSecretToken("foo"),
		apmproxy.WithAPIKey("bar"),
		apmproxy.WithReceiverAddress("127.0.0.1:1234"),
		apmproxy.WithReceiverTimeout(15*time.Second),
		apmproxy.WithLogger(zaptest.NewLogger(t).Sugar()),
	)
	require.NoError(t, err)

	require.NoError(t, apmClient.StartReceiver())
	defer func() {
		require.NoError(t, apmClient.Shutdown())
	}()

	url := "http://127.0.0.1:1234"

	// Create a request to send to the extension
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)

	// Send the request to the extension
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestInfoProxyUnreachable(t *testing.T) {
	// Create apm server and handler
	apmServer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	// Shutdown
	apmServer.Close()

	// Create extension config and start the server
	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL(apmServer.URL),
		apmproxy.WithSecretToken("foo"),
		apmproxy.WithAPIKey("bar"),
		apmproxy.WithReceiverAddress("127.0.0.1:1234"),
		apmproxy.WithReceiverTimeout(15*time.Second),
		apmproxy.WithLogger(zaptest.NewLogger(t).Sugar()),
	)
	require.NoError(t, err)

	require.NoError(t, apmClient.StartReceiver())
	defer func() {
		require.NoError(t, apmClient.Shutdown())
	}()

	url := "http://127.0.0.1:1234"

	// Create a request to send to the extension
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)

	// Send the request to the extension
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Make sure we don't get a 200 OK
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
}

func Test_handleInfoRequest(t *testing.T) {
	headers := map[string]string{"Authorization": "test-value"}
	// Copied from https://github.com/elastic/apm-server/blob/master/testdata/intake-v2/transactions.ndjson.
	agentRequestBody := `{"metadata": {"service": {"name": "1234_service-12a3","node": {"configured_name": "node-123"},"version": "5.1.3","environment": "staging","language": {"name": "ecmascript","version": "8"},"runtime": {"name": "node","version": "8.0.0"},"framework": {"name": "Express","version": "1.2.3"},"agent": {"name": "elastic-node","version": "3.14.0"}},"user": {"id": "123user", "username": "bar", "email": "bar@user.com"}, "labels": {"tag0": null, "tag1": "one", "tag2": 2}, "process": {"pid": 1234,"ppid": 6789,"title": "node","argv": ["node","server.js"]},"system": {"hostname": "prod1.example.com","architecture": "x64","platform": "darwin", "container": {"id": "container-id"}, "kubernetes": {"namespace": "namespace1", "pod": {"uid": "pod-uid", "name": "pod-name"}, "node": {"name": "node-name"}}},"cloud":{"account":{"id":"account_id","name":"account_name"},"availability_zone":"cloud_availability_zone","instance":{"id":"instance_id","name":"instance_name"},"machine":{"type":"machine_type"},"project":{"id":"project_id","name":"project_name"},"provider":"cloud_provider","region":"cloud_region","service":{"name":"lambda"}}}}
{"transaction": { "id": "945254c567a5417e", "trace_id": "0123456789abcdef0123456789abcdef", "parent_id": "abcdefabcdef01234567", "type": "request", "duration": 32.592981,  "span_count": { "started": 43 }}}
{"transaction": { "id": "00xxxxFFaaaa1234", "trace_id": "0123456789abcdef0123456789abcdef", "name": "amqp receive", "parent_id": "abcdefabcdef01234567", "type": "messaging", "duration": 3, "span_count": { "started": 1 }, "context": {"message": {"queue": { "name": "new_users"}, "age":{ "ms": 1577958057123}, "headers": {"user_id": "1ax3", "involved_services": ["user", "auth"]}, "body": "user created", "routing_key": "user-created-transaction"}},"session":{"id":"sunday","sequence":123}}}
`

	// Create extension config
	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL("https://example.com"),
		apmproxy.WithSecretToken("foo"),
		apmproxy.WithAPIKey("bar"),
		apmproxy.WithReceiverAddress("127.0.0.1:1234"),
		apmproxy.WithReceiverTimeout(15*time.Second),
		apmproxy.WithLogger(zaptest.NewLogger(t).Sugar()),
	)
	require.NoError(t, err)

	// Start extension server
	require.NoError(t, apmClient.StartReceiver())
	defer func() {
		require.NoError(t, apmClient.Shutdown())
	}()

	// Create a request to send to the extension
	url := "http://127.0.0.1:1234/intake/v2/events"
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(agentRequestBody))
	require.NoError(t, err)
	// Add headers to the request
	for name, value := range headers {
		req.Header.Add(name, value)
	}

	time.Sleep(5 * time.Second)

	// Send the request to the extension
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
}

func Test_handleIntakeV2EventsQueryParam(t *testing.T) {
	body := []byte(`{"metadata": {}`)

	// Create apm server and handler
	apmServer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	defer apmServer.Close()

	// Create extension config and start the server
	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL(apmServer.URL),
		apmproxy.WithReceiverAddress("127.0.0.1:1234"),
		apmproxy.WithReceiverTimeout(15*time.Second),
		apmproxy.WithLogger(zaptest.NewLogger(t).Sugar()),
	)
	require.NoError(t, err)
	require.NoError(t, apmClient.StartReceiver())
	defer func() {
		require.NoError(t, apmClient.Shutdown())
	}()

	url := "http://127.0.0.1:1234/intake/v2/events?flushed=true"

	// Create a request to send to the extension
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	require.NoError(t, err)

	// Send the request to the extension
	client := &http.Client{}
	go func() {
		resp, err := client.Do(req)
		assert.NoError(t, err)
		if err == nil {
			resp.Body.Close()
		}
	}()

	select {
	case <-apmClient.WaitForFlush():
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for server to send flush signal")
	}
}

func Test_handleIntakeV2EventsNoQueryParam(t *testing.T) {
	body := []byte(`{"metadata": {}`)

	// Create apm server and handler
	apmServer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	defer apmServer.Close()

	// Create extension config and start the server
	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL(apmServer.URL),
		apmproxy.WithReceiverAddress("127.0.0.1:1234"),
		apmproxy.WithReceiverTimeout(15*time.Second),
		apmproxy.WithLogger(zaptest.NewLogger(t).Sugar()),
	)
	require.NoError(t, err)
	require.NoError(t, apmClient.StartReceiver())
	defer func() {
		require.NoError(t, apmClient.Shutdown())
	}()

	url := "http://127.0.0.1:1234/intake/v2/events"

	// Create a request to send to the extension
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Logf("Could not create request")
	}

	// Send the request to the extension
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	select {
	case <-apmClient.AgentDataChannel:
	case <-time.After(1 * time.Second):
		t.Log("Timed out waiting for server to send FuncDone signal")
		t.Fail()
	}
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
}

func Test_handleIntakeV2EventsQueryParamEmptyData(t *testing.T) {
	body := []byte(``)

	// Create apm server and handler
	apmServer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	defer apmServer.Close()

	// Create extension config and start the server
	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL(apmServer.URL),
		apmproxy.WithReceiverAddress("127.0.0.1:1234"),
		apmproxy.WithReceiverTimeout(15*time.Second),
		apmproxy.WithLogger(zaptest.NewLogger(t).Sugar()),
	)
	require.NoError(t, err)
	require.NoError(t, apmClient.StartReceiver())
	defer func() {
		require.NoError(t, apmClient.Shutdown())
	}()

	url := "http://127.0.0.1:1234/intake/v2/events?flushed=true"

	// Create a request to send to the extension
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	require.NoError(t, err)

	// Send the request to the extension
	client := &http.Client{}
	go func() {
		resp, err := client.Do(req)
		assert.NoError(t, err)
		if err == nil {
			resp.Body.Close()
		}
	}()

	select {
	case <-apmClient.WaitForFlush():
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for server to send flush signal")
	}
}

func TestWithVerifyCerts(t *testing.T) {
	headers := map[string]string{"Authorization": "test-value"}
	clientConnected := false

	// Create apm server and handler
	apmServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("test", "header")
		_, err := w.Write([]byte(`{"foo": "bar"}`))
		assert.NoError(t, err)
		clientConnected = true
	}))
	defer apmServer.Close()

	// Create extension config and start the server
	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL(apmServer.URL),
		apmproxy.WithSecretToken("foo"),
		apmproxy.WithAPIKey("bar"),
		apmproxy.WithReceiverAddress("127.0.0.1:1234"),
		apmproxy.WithReceiverTimeout(15*time.Second),
		apmproxy.WithLogger(zaptest.NewLogger(t).Sugar()),
		apmproxy.WithVerifyCerts(false),
	)
	require.NoError(t, err)

	require.NoError(t, apmClient.StartReceiver())
	defer func() {
		require.NoError(t, apmClient.Shutdown())
	}()

	url := "http://127.0.0.1:1234"

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
	require.NoError(t, resp.Body.Close())

	require.True(t, clientConnected, "The apm proxy did not connect to the tls server.")
}

func TestWithRootCerts(t *testing.T) {
	headers := map[string]string{"Authorization": "test-value"}
	clientConnected := false

	// Create apm server and handler
	apmServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("test", "header")
		_, err := w.Write([]byte(`{"foo": "bar"}`))
		assert.NoError(t, err)
		clientConnected = true
	}))
	defer apmServer.Close()

	pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: apmServer.Certificate().Raw})

	// Create extension config and start the server
	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL(apmServer.URL),
		apmproxy.WithSecretToken("foo"),
		apmproxy.WithAPIKey("bar"),
		apmproxy.WithReceiverAddress("127.0.0.1:1234"),
		apmproxy.WithReceiverTimeout(15*time.Second),
		apmproxy.WithLogger(zaptest.NewLogger(t).Sugar()),
		apmproxy.WithRootCerts(string(pemCert)),
	)
	require.NoError(t, err)

	require.NoError(t, apmClient.StartReceiver())
	defer func() {
		require.NoError(t, apmClient.Shutdown())
	}()

	url := "http://127.0.0.1:1234"

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
	require.NoError(t, resp.Body.Close())

	require.True(t, clientConnected, "The apm proxy did not connect to the tls server.")
}
