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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/elastic/apm-aws-lambda/version"
	"go.uber.org/zap"
)

// RegisterResponse is the body of the response for /register
type RegisterResponse struct {
	FunctionName    string `json:"functionName"`
	FunctionVersion string `json:"functionVersion"`
	Handler         string `json:"handler"`
}

// NextEventResponse is the response for /event/next
type NextEventResponse struct {
	Timestamp          time.Time `json:"timestamp,omitempty"`
	EventType          EventType `json:"eventType"`
	ShutdownReason     string    `json:"shutdownReason,omitempty"`
	DeadlineMs         int64     `json:"deadlineMs"`
	RequestID          string    `json:"requestId"`
	InvokedFunctionArn string    `json:"invokedFunctionArn"`
	Tracing            Tracing   `json:"tracing"`
}

// Tracing is part of the response for /event/next
type Tracing struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// StatusResponse is the body of the response for /init/error and /exit/error
type StatusResponse struct {
	Status string `json:"status"`
}

// EventType represents the type of events recieved from /event/next
type EventType string

const (
	// Invoke is a lambda invoke
	Invoke EventType = "INVOKE"

	// Shutdown is a shutdown event for the environment
	Shutdown EventType = "SHUTDOWN"

	extensionNameHeader      = "Lambda-Extension-Name"
	extensionIdentiferHeader = "Lambda-Extension-Identifier"
	extensionErrorType       = "Lambda-Extension-Function-Error-Type"
)

// Client is a simple Client for the Lambda Extensions API
type Client struct {
	baseURL     string
	httpClient  *http.Client
	ExtensionID string
	logger      *zap.SugaredLogger
}

// NewClient returns a Lambda Extensions API Client
func NewClient(awsLambdaRuntimeAPI string, logger *zap.SugaredLogger) *Client {
	baseURL := fmt.Sprintf("http://%s/2020-01-01/extension", awsLambdaRuntimeAPI)
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{},
		logger:     logger,
	}
}

// Register will register the extension with the Extensions API
func (e *Client) Register(ctx context.Context, filename string) (*RegisterResponse, error) {
	const action = "/register"
	url := e.baseURL + action

	reqBody, err := json.Marshal(map[string]interface{}{
		"events": []EventType{Invoke, Shutdown},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal register request body: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create register request: %w", err)
	}
	httpReq.Header.Set(extensionNameHeader, filename)
	httpReq.Header.Set("User-Agent", version.UserAgent)
	httpRes, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("extension register request failed: %w", err)
	}
	defer httpRes.Body.Close()

	if httpRes.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("extension register request failed with status %s", httpRes.Status)
	}
	res := RegisterResponse{}
	if err := json.NewDecoder(httpRes.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode register response body: %w", err)
	}
	e.ExtensionID = httpRes.Header.Get(extensionIdentiferHeader)
	e.logger.Debugf("ExtensionID : %s", e.ExtensionID)
	return &res, nil
}

// NextEvent blocks while long polling for the next lambda invoke or shutdown
func (e *Client) NextEvent(ctx context.Context) (*NextEventResponse, error) {
	const action = "/event/next"
	url := e.baseURL + action

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create next event request: %w", err)
	}
	httpReq.Header.Set(extensionIdentiferHeader, e.ExtensionID)
	httpReq.Header.Set("User-Agent", version.UserAgent)
	httpRes, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("next event request failed: %w", err)
	}
	defer httpRes.Body.Close()

	if httpRes.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("next event request failed with status %s", httpRes.Status)
	}
	res := NextEventResponse{}
	if err := json.NewDecoder(httpRes.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode next event response body: %w", err)
	}
	return &res, nil
}

// InitError reports an initialization error to the platform. Call it when you registered but failed to initialize
func (e *Client) InitError(ctx context.Context, errorType string) (*StatusResponse, error) {
	const action = "/init/error"
	url := e.baseURL + action

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create init error request: %w", err)
	}
	httpReq.Header.Set(extensionIdentiferHeader, e.ExtensionID)
	httpReq.Header.Set(extensionErrorType, errorType)
	httpReq.Header.Set("User-Agent", version.UserAgent)
	httpRes, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("initialization error request failed: %w", err)
	}
	defer httpRes.Body.Close()

	if httpRes.StatusCode > 299 {
		return nil, fmt.Errorf("initialization error request failed with status %s", httpRes.Status)
	}
	res := StatusResponse{}
	if err := json.NewDecoder(httpRes.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode init error response body: %w", err)
	}
	return &res, nil
}

// ExitError reports an error to the platform before exiting. Call it when you encounter an unexpected failure
func (e *Client) ExitError(ctx context.Context, errorType string) (*StatusResponse, error) {
	const action = "/exit/error"
	url := e.baseURL + action

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create exit error request: %w", err)
	}
	httpReq.Header.Set(extensionIdentiferHeader, e.ExtensionID)
	httpReq.Header.Set(extensionErrorType, errorType)
	httpReq.Header.Set("User-Agent", version.UserAgent)
	httpRes, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("exit error request failed: %w", err)
	}
	defer httpRes.Body.Close()

	if httpRes.StatusCode > 299 {
		return nil, fmt.Errorf("exit error request failed with status %s", httpRes.Status)
	}
	res := StatusResponse{}
	if err := json.NewDecoder(httpRes.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode exit error response body: %w", err)
	}
	return &res, nil
}
