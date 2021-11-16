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
	"log"
	"net/http"
)

type AgentData struct {
	Data            []byte
	ContentEncoding string
}

// URL: http://server/
func handleInfoRequest(apmServerUrl string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		client := &http.Client{}

		req, err := http.NewRequest(r.Method, apmServerUrl, nil)
		//forward every header received
		for name, values := range r.Header {
			// Loop over all values for the name.
			for _, value := range values {
				req.Header.Set(name, value)
			}
		}
		if err != nil {
			log.Printf("could not create request object for %s:%s: %v", r.Method, apmServerUrl, err)
			return
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("error forwarding info request (`/`) to APM Server: %v", err)
			return
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Printf("could not read info request response to APM Server: %v", err)
			return
		}

		// send status code
		w.WriteHeader(resp.StatusCode)

		// send every header received
		for name, values := range resp.Header {
			// Loop over all values for the name.
			for _, value := range values {
				w.Header().Add(name, value)
			}
		}
		// send body
		w.Write([]byte(body))
	}
}

// URL: http://server/intake/v2/events
func handleIntakeV2Events(agentDataChan chan AgentData) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("ok"))

		if r.Body == nil {
			log.Println("No body in agent request")
			return
		}

		rawBytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Println("Could not read bytes from agent request body")
			return
		}

		agentData := AgentData{
			Data:            rawBytes,
			ContentEncoding: r.Header.Get("Content-Encoding"),
		}
		log.Println("Adding agent data to buffer to be sent to apm server")
		agentDataChan <- agentData
	}
}
