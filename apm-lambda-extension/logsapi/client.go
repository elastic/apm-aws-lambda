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

package logsapi

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// ClientOption is a config option for a Client.
type ClientOption func(*Client)

// Client is the client used to subscribe to the Logs API.
type Client struct {
	httpClient     *http.Client
	logsAPIBaseURL string
	logsChannel    chan LogEvent
	listenerAddr   string
	server         *http.Server
}

// NewClient returns a new Client with the given URL.
func NewClient(opts ...ClientOption) (*Client, error) {
	c := Client{
		server:     &http.Server{},
		httpClient: &http.Client{},
	}

	for _, opt := range opts {
		opt(&c)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleLogEventsRequest(c.logsChannel))

	c.server.Handler = mux

	if c.logsAPIBaseURL == "" {
		return nil, errors.New("logs api base url cannot be empty")
	}

	return &c, nil
}

// StartService starts the HTTP server listening for log events and subscribes to the Logs API.
func (lc *Client) StartService(eventTypes []EventType, extensionID string) error {
	addr, err := lc.startHTTPServer()
	if err != nil {
		return err
	}

	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		lc.Shutdown()
		return fmt.Errorf("failed to retrieve port from address %s: %w", addr, err)
	}

	host, _, err := net.SplitHostPort(lc.listenerAddr)
	if err != nil {
		lc.Shutdown()
		return fmt.Errorf("failed to retrieve host from address %s: %w", lc.listenerAddr, err)
	}

	uri := fmt.Sprintf("http://%s", net.JoinHostPort(host, port))

	if err := lc.subscribe(eventTypes, extensionID, uri); err != nil {
		lc.Shutdown()
		return err
	}

	return nil
}

// Shutdown shutdowns the log service gracefully.
func (lc *Client) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return lc.server.Shutdown(ctx)
}
