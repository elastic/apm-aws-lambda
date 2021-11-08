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
	"net"
	"net/http"
	"time"
)

type serverHandler struct {
	data   chan AgentData
	config *extensionConfig
}

func (handler *serverHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/intake/v2/events" {
		handleIntakeV2Events(handler, w, r)
		return
	}

	if r.URL.Path == "/" {
		handleInfoRequest(handler, w, r)
		return
	}

	// if we have not yet returned, 404
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("404"))

}

func NewHttpServer(dataChannel chan AgentData, config *extensionConfig) *http.Server {
	var handler = serverHandler{data: dataChannel, config: config}
	timeout := time.Duration(config.dataReceiverTimeoutSeconds) * time.Second
	s := &http.Server{
		Addr:           config.dataReceiverServerPort,
		Handler:        &handler,
		ReadTimeout:    timeout,
		WriteTimeout:   timeout,
		MaxHeaderBytes: 1 << 20,
	}

	addr := s.Addr
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return s
	}
	go s.Serve(ln)

	return s
}
