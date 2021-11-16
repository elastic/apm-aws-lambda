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
	"log"
	"net/http"
)

var extensionServer *http.Server

func StartHttpServer(agentDataChan chan AgentData, config *extensionConfig) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleInfoRequest(config.apmServerUrl))
	mux.HandleFunc("/intake/v2/events", handleIntakeV2Events(agentDataChan))
	extensionServer = &http.Server{Addr: config.dataReceiverServerPort, Handler: mux}

	go func() {
		log.Printf("Extension liistening for apm data on %s", extensionServer.Addr)
		err := extensionServer.ListenAndServe()
		if err != http.ErrServerClosed {
			log.Printf("Unexpected stop on Extension Server: %v", err)
		} else {
			log.Printf("Extension Server closed %v", err)
		}
	}()
}
