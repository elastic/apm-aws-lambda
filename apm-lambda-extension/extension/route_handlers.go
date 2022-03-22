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
	"net/http/httputil"
	"net/url"
)

type AgentData struct {
	Data            []byte
	ContentEncoding string
}

var AgentDoneSignal chan struct{}

// URL: http://server/
func handleInfoRequest(apmServerUrl string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		// Init reverse proxy
		parsedApmServerUrl, err := url.Parse(apmServerUrl)
		if err != nil {
			Log.Errorf("could not parse APM server URL: %v", err)
			return
		}
		reverseProxy := httputil.NewSingleHostReverseProxy(parsedApmServerUrl)

		// Process request (the Golang doc suggests removing pre-existing any pre-existing X-Forwarded-For header coming
		// from the client or an untrusted proxy to prevent IP spoofing : https://pkg.go.dev/net/http/httputil#ReverseProxy
		r.Header.Del("X-Forwarded-For")

		// Update headers to allow for SSL redirection
		r.URL.Host = parsedApmServerUrl.Host
		r.URL.Scheme = parsedApmServerUrl.Scheme
		r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
		r.Host = parsedApmServerUrl.Host

		// Forward request to the APM server
		reverseProxy.ServeHTTP(w, r)
	}
}

// URL: http://server/intake/v2/events
func handleIntakeV2Events(agentDataChan chan AgentData) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		rawBytes, err := ioutil.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			Log.Errorf("Could not read agent intake request body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if len(rawBytes) > 0 {
			agentData := AgentData{
				Data:            rawBytes,
				ContentEncoding: r.Header.Get("Content-Encoding"),
			}

			EnqueueAPMData(agentDataChan, agentData)
		}

		if len(r.URL.Query()["flushed"]) > 0 && r.URL.Query()["flushed"][0] == "true" {
			AgentDoneSignal <- struct{}{}
		}

		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("ok"))
	}
}
