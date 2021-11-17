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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

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
		w.Write([]byte(`{"foo": "bar"}`))
	}))
	defer apmServer.Close()

	// Create extension config and start the server
	dataChannel := make(chan AgentData, 100)
	config := extensionConfig{
		apmServerUrl:               apmServer.URL,
		apmServerSecretToken:       "foo",
		apmServerApiKey:            "bar",
		dataReceiverServerPort:     ":4567",
		dataReceiverTimeoutSeconds: 15,
	}

	StartHttpServer(dataChannel, &config)
	defer agentDataServer.Close()

	// Create a request to send to the extension
	client := &http.Client{}
	url := "http://localhost" + agentDataServer.Addr
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Logf("Could not create request")
	}
	for name, value := range headers {
		req.Header.Add(name, value)
	}

	// Send the request to the extension
	resp, err := client.Do(req)
	if err != nil {
		t.Logf("Error fetching %s, [%v]", agentDataServer.Addr, err)
		t.Fail()
	} else {
		body, _ := ioutil.ReadAll(resp.Body)
		assert.Equal(t, string(body), wantResp)
		resp.Body.Close()
	}
}
