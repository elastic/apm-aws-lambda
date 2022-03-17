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
	"io"
	"io/ioutil"
	"net/http"
)

type AgentData struct {
	Data            []byte
	ContentEncoding string
}

var AgentDoneSignal chan struct{}

// URL: http://server/
func handleInfoRequest(apmServerUrl string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		client := &http.Client{}

		req, err := http.NewRequest(r.Method, apmServerUrl, nil)
		if err != nil {
			Log.Errorf("could not create request object for %s:%s: %v", r.Method, apmServerUrl, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		//forward every header received
		for name, values := range r.Header {
			// Loop over all values for the name.
			for _, value := range values {
				req.Header.Set(name, value)
			}
		}

		// Send request to apm server
		serverResp, err := client.Do(req)
		if err != nil {
			Log.Errorf("error forwarding info request (`/`) to APM Server: %v", err)
			return
		}
		defer serverResp.Body.Close()

		// If WriteHeader is not called explicitly, the first call to Write
		// will trigger an implicit WriteHeader(http.StatusOK).
		if serverResp.StatusCode != 200 {
			w.WriteHeader(serverResp.StatusCode)
		}

		// send every header received
		for name, values := range serverResp.Header {
			// Loop over all values for the name.
			for _, value := range values {
				w.Header().Add(name, value)
			}
		}

		// copy body to request sent back to the agent
		_, err = io.Copy(w, serverResp.Body)
		if err != nil {
			Log.Errorf("could not read info request response to APM Server: %v", err)
			return
		}
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

			select {
			case agentDataChan <- agentData:
				Log.Debug("Adding agent data to buffer to be sent to apm server")
			default:
				Log.Warn("Channel full: dropping a subset of agent request data")
			}
		}

		if len(r.URL.Query()["flushed"]) > 0 && r.URL.Query()["flushed"][0] == "true" {
			AgentDoneSignal <- struct{}{}
		}

		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("ok"))
	}
}
