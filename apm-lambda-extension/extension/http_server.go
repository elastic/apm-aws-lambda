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
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

type serverHandler struct {
	data   chan []byte
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

func NewHttpServer(dataChannel chan []byte, config *extensionConfig) *http.Server {
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

func getDecompressedBytesFromRequest(req *http.Request) ([]byte, error) {
	var rawBytes []byte
	if req.Body != nil {
		rawBytes, _ = ioutil.ReadAll(req.Body)
	}

	switch req.Header.Get("Content-Encoding") {
	case "deflate":
		reader := bytes.NewReader([]byte(rawBytes))
		zlibreader, err := zlib.NewReader(reader)
		if err != nil {
			return nil, fmt.Errorf("could not create zlib.NewReader: %v", err)
		}
		bodyBytes, err := ioutil.ReadAll(zlibreader)
		if err != nil {
			return nil, fmt.Errorf("could not read from zlib reader using ioutil.ReadAll: %v", err)
		}
		return bodyBytes, nil
	case "gzip":
		reader := bytes.NewReader([]byte(rawBytes))
		zlibreader, err := gzip.NewReader(reader)
		if err != nil {
			return nil, fmt.Errorf("could not create gzip.NewReader: %v", err)
		}
		bodyBytes, err := ioutil.ReadAll(zlibreader)
		if err != nil {
			return nil, fmt.Errorf("could not read from gzip reader using ioutil.ReadAll: %v", err)
		}
		return bodyBytes, nil
	default:
		return rawBytes, nil
	}
}
