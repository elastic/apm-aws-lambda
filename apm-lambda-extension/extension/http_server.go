// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package extension

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

// StartHttpServer starts the server listening for APM agent data.
func StartHttpServer(ctx context.Context, transport *ApmServerTransport) (agentDataServer *http.Server, err error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleInfoRequest(ctx, transport))
	mux.HandleFunc("/intake/v2/events", handleIntakeV2Events(transport))
	timeout := time.Duration(transport.config.dataReceiverTimeoutSeconds) * time.Second
	server := &http.Server{
		Addr:           transport.config.dataReceiverServerPort,
		Handler:        mux,
		ReadTimeout:    timeout,
		WriteTimeout:   timeout,
		MaxHeaderBytes: 1 << 20,
	}

	ln, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return
	}

	go func() {
		Log.Infof("Extension listening for apm data on %s", server.Addr)
		if err = server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			Log.Errorf("received error from http.Serve(): %v", err)
		} else {
			Log.Debug("server closed")
		}
	}()
	return server, nil
}
