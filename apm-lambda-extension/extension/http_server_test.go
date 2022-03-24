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
	"bytes"
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gotest.tools/assert"
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
		if err != nil {
			t.Fail()
			return
		}
	}))
	defer apmServer.Close()

	// Create extension config and start the server
	dataChannel := make(chan AgentData, 100)
	config := extensionConfig{
		apmServerUrl:               apmServer.URL,
		apmServerSecretToken:       "foo",
		apmServerApiKey:            "bar",
		dataReceiverServerPort:     ":1234",
		dataReceiverTimeoutSeconds: 15,
	}

	err := StartHttpServer(dataChannel, &config)
	if err != nil {
		t.Fail()
		return
	}
	defer agentDataServer.Close()

	hosts, _ := net.LookupHost("localhost")
	url := "http://" + hosts[0] + ":1234"

	// Create a request to send to the extension
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Logf("Could not create request")
	}
	for name, value := range headers {
		req.Header.Add(name, value)
	}

	// Send the request to the extension
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Logf("Error fetching %s, [%v]", agentDataServer.Addr, err)
		t.Fail()
	} else {
		body, _ := ioutil.ReadAll(resp.Body)
		assert.Equal(t, string(body), wantResp)
		assert.Equal(t, "header", resp.Header.Get("test"))
		resp.Body.Close()
	}
}

func TestInfoProxyErrorStatusCode(t *testing.T) {
	// Create apm server and handler
	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	defer apmServer.Close()

	// Create extension config and start the server
	dataChannel := make(chan AgentData, 100)
	config := extensionConfig{
		apmServerUrl:               apmServer.URL,
		apmServerSecretToken:       "foo",
		apmServerApiKey:            "bar",
		dataReceiverServerPort:     ":1234",
		dataReceiverTimeoutSeconds: 15,
	}

	err := StartHttpServer(dataChannel, &config)
	if err != nil {
		t.Fail()
		return
	}
	defer agentDataServer.Close()

	hosts, _ := net.LookupHost("localhost")
	url := "http://" + hosts[0] + ":1234"

	// Create a request to send to the extension
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Logf("Could not create request")
	}

	// Send the request to the extension
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Logf("Error fetching %s, [%v]", agentDataServer.Addr, err)
		t.Fail()
	} else {
		assert.Equal(t, 401, resp.StatusCode)
	}
}

func Test_handleInfoRequest(t *testing.T) {
	headers := map[string]string{"Authorization": "test-value"}
	// Copied from https://github.com/elastic/apm-server/blob/master/testdata/intake-v2/transactions.ndjson.
	agentRequestBody := `{"metadata": {"service": {"name": "1234_service-12a3","node": {"configured_name": "node-123"},"version": "5.1.3","environment": "staging","language": {"name": "ecmascript","version": "8"},"runtime": {"name": "node","version": "8.0.0"},"framework": {"name": "Express","version": "1.2.3"},"agent": {"name": "elastic-node","version": "3.14.0"}},"user": {"id": "123user", "username": "bar", "email": "bar@user.com"}, "labels": {"tag0": null, "tag1": "one", "tag2": 2}, "process": {"pid": 1234,"ppid": 6789,"title": "node","argv": ["node","server.js"]},"system": {"hostname": "prod1.example.com","architecture": "x64","platform": "darwin", "container": {"id": "container-id"}, "kubernetes": {"namespace": "namespace1", "pod": {"uid": "pod-uid", "name": "pod-name"}, "node": {"name": "node-name"}}},"cloud":{"account":{"id":"account_id","name":"account_name"},"availability_zone":"cloud_availability_zone","instance":{"id":"instance_id","name":"instance_name"},"machine":{"type":"machine_type"},"project":{"id":"project_id","name":"project_name"},"provider":"cloud_provider","region":"cloud_region","service":{"name":"lambda"}}}}
{"transaction": { "id": "945254c567a5417e", "trace_id": "0123456789abcdef0123456789abcdef", "parent_id": "abcdefabcdef01234567", "type": "request", "duration": 32.592981,  "span_count": { "started": 43 }}}
{"transaction": { "id": "00xxxxFFaaaa1234", "trace_id": "0123456789abcdef0123456789abcdef", "name": "amqp receive", "parent_id": "abcdefabcdef01234567", "type": "messaging", "duration": 3, "span_count": { "started": 1 }, "context": {"message": {"queue": { "name": "new_users"}, "age":{ "ms": 1577958057123}, "headers": {"user_id": "1ax3", "involved_services": ["user", "auth"]}, "body": "user created", "routing_key": "user-created-transaction"}},"session":{"id":"sunday","sequence":123}}}
`

	// Create extension config
	dataChannel := make(chan AgentData, 100)
	config := extensionConfig{
		apmServerSecretToken:       "foo",
		apmServerApiKey:            "bar",
		dataReceiverServerPort:     ":1234",
		dataReceiverTimeoutSeconds: 15,
	}

	// Start extension server
	err := StartHttpServer(dataChannel, &config)
	if err != nil {
		t.Fail()
		return
	}
	defer agentDataServer.Close()

	// Create a request to send to the extension
	hosts, _ := net.LookupHost("localhost")
	url := "http://" + hosts[0] + ":1234/intake/v2/events"
	req, err := http.NewRequest("POST", url, strings.NewReader(agentRequestBody))
	if err != nil {
		t.Logf("Could not create request")
	}
	// Add headers to the request
	for name, value := range headers {
		req.Header.Add(name, value)
	}

	// Send the request to the extension
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Logf("Error fetching %s, [%v]", agentDataServer.Addr, err)
		t.Fail()
	} else {
		assert.Equal(t, 202, resp.StatusCode)
	}
}

type errReader int

func (errReader) Read(_ []byte) (int, error) {
	return 0, errors.New("test error")
}

func Test_handleInfoRequestInvalidBody(t *testing.T) {
	testChan := make(chan AgentData)
	mux := http.NewServeMux()
	urlPath := "/intake/v2/events"
	mux.HandleFunc(urlPath, handleIntakeV2Events(testChan))
	req := httptest.NewRequest(http.MethodGet, urlPath, errReader(0))
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func Test_handleIntakeV2EventsQueryParam(t *testing.T) {
	body := []byte(`{"metadata": {}`)

	AgentDoneSignal = make(chan struct{})

	// Create apm server and handler
	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer apmServer.Close()

	// Create extension config and start the server
	dataChannel := make(chan AgentData, 100)
	config := extensionConfig{
		apmServerUrl:               apmServer.URL,
		dataReceiverServerPort:     ":1234",
		dataReceiverTimeoutSeconds: 15,
	}

	err := StartHttpServer(dataChannel, &config)
	if err != nil {
		t.Fail()
		return
	}
	defer agentDataServer.Close()

	hosts, _ := net.LookupHost("localhost")
	url := "http://" + hosts[0] + ":1234/intake/v2/events?flushed=true"

	// Create a request to send to the extension
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		t.Logf("Could not create request")
	}

	// Send the request to the extension
	client := &http.Client{}
	go func() {
		_, err := client.Do(req)
		if err != nil {
			t.Logf("Error fetching %s, [%v]", agentDataServer.Addr, err)
			t.Fail()
		}
	}()

	timer := time.NewTimer(1 * time.Second)
	defer timer.Stop()

	select {
	case <-AgentDoneSignal:
		<-dataChannel
	case <-timer.C:
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

	// Create extension config and start the server
	dataChannel := make(chan AgentData, 100)
	config := extensionConfig{
		apmServerUrl:               apmServer.URL,
		dataReceiverServerPort:     ":1234",
		dataReceiverTimeoutSeconds: 15,
	}

	err := StartHttpServer(dataChannel, &config)
	if err != nil {
		t.Fail()
		return
	}
	defer agentDataServer.Close()

	hosts, _ := net.LookupHost("localhost")
	url := "http://" + hosts[0] + ":1234/intake/v2/events"

	// Create a request to send to the extension
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		t.Logf("Could not create request")
	}

	// Send the request to the extension
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Logf("Error fetching %s, [%v]", agentDataServer.Addr, err)
		t.Fail()
	}
	<-dataChannel
	assert.Equal(t, 202, resp.StatusCode)
}

func Test_handleIntakeV2EventsQueryParamEmptyData(t *testing.T) {
	body := []byte(``)

	AgentDoneSignal = make(chan struct{})

	// Create apm server and handler
	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer apmServer.Close()

	// Create extension config and start the server
	dataChannel := make(chan AgentData, 100)
	config := extensionConfig{
		apmServerUrl:               apmServer.URL,
		dataReceiverServerPort:     ":1234",
		dataReceiverTimeoutSeconds: 15,
	}

	err := StartHttpServer(dataChannel, &config)
	if err != nil {
		t.Fail()
		return
	}
	defer agentDataServer.Close()

	hosts, _ := net.LookupHost("localhost")
	url := "http://" + hosts[0] + ":1234/intake/v2/events?flushed=true"

	// Create a request to send to the extension
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		t.Logf("Could not create request")
	}

	// Send the request to the extension
	client := &http.Client{}
	go func() {
		_, err := client.Do(req)
		if err != nil {
			t.Logf("Error fetching %s, [%v]", agentDataServer.Addr, err)
			t.Fail()
		}
	}()

	timer := time.NewTimer(1 * time.Second)
	defer timer.Stop()

	select {
	case <-AgentDoneSignal:
	case <-timer.C:
		t.Log("Timed out waiting for server to send FuncDone signal")
		t.Fail()
	}
}
