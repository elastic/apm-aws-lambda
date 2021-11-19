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
	"net"
	"net/http"
	"time"
)

var agentDataServer *http.Server

func StartHttpServer(agentDataChan chan AgentData, config *extensionConfig) (err error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleInfoRequest(config.apmServerUrl))
	mux.HandleFunc("/intake/v2/events", handleIntakeV2Events(agentDataChan))
	timeout := time.Duration(config.dataReceiverTimeoutSeconds) * time.Second
	agentDataServer = &http.Server{
		Addr:           config.dataReceiverServerPort,
		Handler:        mux,
		ReadTimeout:    timeout,
		WriteTimeout:   timeout,
		MaxHeaderBytes: 1 << 20,
	}

	ln, err := net.Listen("tcp", agentDataServer.Addr)
	if err != nil {
		return
	}

	go func() {
		log.Printf("Extension listening for apm data on %s", agentDataServer.Addr)
		agentDataServer.Serve(ln)
	}()
	return nil
}
