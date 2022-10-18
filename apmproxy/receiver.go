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

package apmproxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

// APMDataType represents source of APMData
type APMDataType string

const (
	// Agent data type represents APMData collected from APM agents
	Agent APMDataType = "agent"
	// Lambda data type represents APMData collected from logs API
	Lambda APMDataType = "lambda"
)

// APMData represents data to be sent to APMServer. `Agent` type
// data will have `metadata` as ndjson whereas `lambda` type data
// will be without metadata.
type APMData struct {
	Data            []byte
	Type            APMDataType
	ContentEncoding string
}

// StartReceiver starts the server listening for APM agent data.
func (c *Client) StartReceiver() error {
	mux := http.NewServeMux()

	handleInfoRequest, err := c.handleInfoRequest()
	if err != nil {
		return err
	}

	mux.HandleFunc("/", handleInfoRequest)
	mux.HandleFunc("/intake/v2/events", c.handleIntakeV2Events())

	c.receiver.Handler = mux

	ln, err := net.Listen("tcp", c.receiver.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen on addr %s", c.receiver.Addr)
	}

	go func() {
		c.logger.Infof("Extension listening for apm data on %s", c.receiver.Addr)
		if err = c.receiver.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			c.logger.Errorf("received error from http.Serve(): %v", err)
		} else {
			c.logger.Debug("server closed")
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
func (c *Client) handleInfoRequest() (func(w http.ResponseWriter, r *http.Request), error) {
	// Init reverse proxy
	parsedApmServerUrl, err := url.Parse(c.serverURL)
	if err != nil {
		return nil, fmt.Errorf("could not parse APM server URL: %w", err)
	}

	reverseProxy := httputil.NewSingleHostReverseProxy(parsedApmServerUrl)

	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	customTransport.ResponseHeaderTimeout = c.client.Timeout
	reverseProxy.Transport = customTransport

	reverseProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		c.UpdateStatus(r.Context(), Failing)
		c.logger.Errorf("Error querying version from the APM server: %v", err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		c.logger.Debug("Handling APM server Info Request")

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
	}, nil
}

// URL: http://server/intake/v2/events
func (c *Client) handleIntakeV2Events() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		c.logger.Debug("Handling APM Data Intake")
		rawBytes, err := io.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			c.logger.Errorf("Could not read agent intake request body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		agentFlushed := r.URL.Query().Get("flushed") == "true"

		agentData := APMData{
			Data:            rawBytes,
			Type:            Agent,
			ContentEncoding: r.Header.Get("Content-Encoding"),
		}

		if len(agentData.Data) != 0 {
			select {
			case c.AgentDataChannel <- agentData:
			default:
				c.logger.Warnf("Channel full: dropping a subset of agent data")
			}
		}

		if agentFlushed {
			c.flushMutex.Lock()

			select {
			case <-c.flushCh:
				// the channel is closed.
				// the extension received at least a flush request already but the
				// data have not been flushed yet.
				// We can reuse the closed channel.
			default:
				// no pending flush requests
				// close the channel to signal a flush request has
				// been received.
				close(c.flushCh)
			}

			c.flushMutex.Unlock()
		}

		w.WriteHeader(http.StatusAccepted)
		if _, err = w.Write([]byte("ok")); err != nil {
			c.logger.Errorf("Failed to send intake response to APM agent : %v", err)
		}
	}
}
