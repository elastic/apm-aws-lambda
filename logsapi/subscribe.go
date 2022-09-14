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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
)

// SubscribeRequest is the request body that is sent to Logs API on subscribe
type SubscribeRequest struct {
	SchemaVersion SchemaVersion      `json:"schemaVersion"`
	LogTypes      []SubscriptionType `json:"types"`
	BufferingCfg  BufferingCfg       `json:"buffering"`
	Destination   Destination        `json:"destination"`
}

// SchemaVersion is the Lambda runtime API schema version
type SchemaVersion string

const (
	SchemaVersion20210318 = "2021-03-18"
	SchemaVersionLatest   = SchemaVersion20210318
)

// BufferingCfg is the configuration set for receiving logs from Logs API. Whichever of the conditions below is met first, the logs will be sent
type BufferingCfg struct {
	// MaxItems is the maximum number of events to be buffered in memory. (default: 10000, minimum: 1000, maximum: 10000)
	MaxItems uint32 `json:"maxItems"`
	// MaxBytes is the maximum size in bytes of the logs to be buffered in memory. (default: 262144, minimum: 262144, maximum: 1048576)
	MaxBytes uint32 `json:"maxBytes"`
	// TimeoutMS is the maximum time (in milliseconds) for a batch to be buffered. (default: 1000, minimum: 100, maximum: 30000)
	TimeoutMS uint32 `json:"timeoutMs"`
}

// Destination is the configuration for listeners who would like to receive logs with HTTP
type Destination struct {
	Protocol   string `json:"protocol"`
	URI        string `json:"URI"`
	HTTPMethod string `json:"method"`
	Encoding   string `json:"encoding"`
}

func (lc *Client) startHTTPServer() (string, error) {
	listener, err := net.Listen("tcp", lc.listenerAddr)
	if err != nil {
		return "", fmt.Errorf("failed to listen on %s: %w", lc.listenerAddr, err)
	}

	addr := listener.Addr().String()

	go func() {
		lc.logger.Infof("Extension listening for Lambda Logs API events on %s", addr)

		if err := lc.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			lc.logger.Errorf("Error upon Logs API server start : %v", err)
		}
	}()

	return addr, nil
}

func (lc *Client) subscribe(types []SubscriptionType, extensionID string, uri string) error {
	data, err := json.Marshal(&SubscribeRequest{
		SchemaVersion: SchemaVersionLatest,
		LogTypes:      types,
		BufferingCfg: BufferingCfg{
			MaxItems:  10000,
			MaxBytes:  262144,
			TimeoutMS: 25,
		},
		Destination: Destination{
			Protocol:   "HTTP",
			URI:        uri,
			HTTPMethod: http.MethodPost,
			Encoding:   "JSON",
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal SubscribeRequest: %w", err)
	}

	url := fmt.Sprintf("%s/2020-08-15/logs", lc.logsAPIBaseURL)
	resp, err := lc.sendRequest(url, data, extensionID)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusAccepted {
		return errors.New("logs API is not supported in this environment")
	}

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("%s failed: %d[%s]", url, resp.StatusCode, resp.Status)
		}

		return fmt.Errorf("%s failed: %d[%s] %s", url, resp.StatusCode, resp.Status, string(body))
	}

	return nil
}

func (lc *Client) sendRequest(url string, data []byte, extensionID string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Lambda-Extension-Identifier", extensionID)

	resp, err := lc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}
