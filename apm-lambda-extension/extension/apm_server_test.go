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
	"compress/gzip"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"gotest.tools/assert"
)

func TestPostToApmServerDataCompressed(t *testing.T) {
	s := "A long time ago in a galaxy far, far away..."

	// Compress the data
	pr, pw := io.Pipe()
	gw, _ := gzip.NewWriterLevel(pw, gzip.BestSpeed)
	go func() {
		gw.Write([]byte(s))
		gw.Close()
		pw.Close()
	}()

	// Create AgentData struct with compressed data
	data, _ := ioutil.ReadAll(pr)
	agentData := AgentData{Data: data, ContentEncoding: "gzip"}

	// Create apm server and handler
	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bytes, _ := ioutil.ReadAll(r.Body)
		assert.Equal(t, string(data), string(bytes))
		assert.Equal(t, "gzip", r.Header.Get("Content-Encoding"))
		w.Write([]byte(`{"foo": "bar"}`))
	}))
	defer apmServer.Close()

	config := extensionConfig{
		apmServerUrl: apmServer.URL + "/",
	}

	err := PostToApmServer(agentData, &config)
	assert.Equal(t, nil, err)
}

func TestPostToApmServerDataNotCompressed(t *testing.T) {
	s := "A long time ago in a galaxy far, far away..."
	body := []byte(s)
	agentData := AgentData{Data: body, ContentEncoding: ""}

	// Compress the data, so it can be compared with what
	// the apm server receives
	pr, pw := io.Pipe()
	gw, _ := gzip.NewWriterLevel(pw, gzip.BestSpeed)
	go func() {
		gw.Write(body)
		gw.Close()
		pw.Close()
	}()

	// Create apm server and handler
	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request_bytes, _ := ioutil.ReadAll(r.Body)
		compressed_bytes, _ := ioutil.ReadAll(pr)
		assert.Equal(t, string(compressed_bytes), string(request_bytes))
		assert.Equal(t, "gzip", r.Header.Get("Content-Encoding"))
		w.Write([]byte(`{"foo": "bar"}`))
	}))
	defer apmServer.Close()

	config := extensionConfig{
		apmServerUrl: apmServer.URL + "/",
	}

	err := PostToApmServer(agentData, &config)
	assert.Equal(t, nil, err)
}
