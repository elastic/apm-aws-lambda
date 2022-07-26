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

package apm

import (
	"context"
	"elastic/apm-lambda-extension/extension"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

type AgentData struct {
	Data            []byte
	ContentEncoding string
}

// StartHttpServer starts the server listening for APM agent data.
func (c *Client) StartReceiver() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", c.handleInfoRequest())
	mux.HandleFunc("/intake/v2/events", c.handleIntakeV2Events())

	c.receiver.Handler = mux

	ln, err := net.Listen("tcp", c.receiver.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen on addr %s", c.receiver.Addr)
	}

	go func() {
		extension.Log.Infof("Extension listening for apm data on %s", c.receiver.Addr)
		if err = c.receiver.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			extension.Log.Errorf("received error from http.Serve(): %v", err)
		} else {
			extension.Log.Debug("server closed")
		}
	}()
	return nil
}

// Shutdown shutdowns the apm receiver gracefully.
func (c *Client) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return c.receiver.Shutdown(ctx)
}

// URL: http://server/
func (c *Client) handleInfoRequest() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		extension.Log.Debug("Handling APM server Info Request")

		// Init reverse proxy
		parsedApmServerUrl, err := url.Parse(c.serverURL)
		if err != nil {
			extension.Log.Errorf("could not parse APM server URL: %v", err)
			return
		}

		reverseProxy := httputil.NewSingleHostReverseProxy(parsedApmServerUrl)

		customTransport := http.DefaultTransport.(*http.Transport).Clone()
		customTransport.ResponseHeaderTimeout = c.dataForwarderTimeout
		reverseProxy.Transport = customTransport

		reverseProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			c.SetApmServerTransportState(r.Context(), Failing)
			extension.Log.Errorf("Error querying version from the APM server: %v", err)
		}

		// Process request (the Golang doc suggests removing any pre-existing X-Forwarded-For header coming
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
func (c *Client) handleIntakeV2Events() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		extension.Log.Debug("Handling APM Data Intake")
		rawBytes, err := io.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			extension.Log.Errorf("Could not read agent intake request body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if len(rawBytes) > 0 {
			agentData := AgentData{
				Data:            rawBytes,
				ContentEncoding: r.Header.Get("Content-Encoding"),
			}

			c.EnqueueAPMData(agentData)
		}

		if len(r.URL.Query()["flushed"]) > 0 && r.URL.Query()["flushed"][0] == "true" {
			c.AgentDoneSignal <- struct{}{}
		}

		w.WriteHeader(http.StatusAccepted)
		if _, err = w.Write([]byte("ok")); err != nil {
			extension.Log.Errorf("Failed to send intake response to APM agent : %v", err)
		}
	}
}
