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

// URL: http://.../
func handleInfoRequest(handler *serverHandler, w http.ResponseWriter, r *http.Request) {
	client := &http.Client{}

	req, err := http.NewRequest(r.Method, handler.config.apmServerUrl, nil)
	if err != nil {
		log.Printf("could create request object for `/` request to APM Server: %v", err)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("err forwarding `/` request to APM Server: %v", err)
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("got response to `/` request to APM Server, but could not read: %v", err)
		return
	}
	log.Printf("%v", handler.config.apmServerUrl)

	// w.Header().Set("Content-Type", "application/json")

	// send status code
	w.WriteHeader(resp.StatusCode)

	// send every header received

	// send body
	w.Write([]byte(body))
}

// URL: /intake/v2/events
func handleIntakeV2Events(handler *serverHandler, w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := getDecompressedBytesFromRequest(r)
	if nil != err {
		log.Printf("could not get decompressed bytes from request body: %v", err)
	} else {
		log.Println("Adding agent data to buffer to be sent to apm server")
		handler.data <- bodyBytes
	}
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("ok"))
}
