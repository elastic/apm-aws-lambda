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
	"bytes"
	"errors"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	defaultDataReceiverTimeout  time.Duration = 15 * time.Second
	defaultDataForwarderTimeout time.Duration = 3 * time.Second
	defaultReceiverAddr                       = ":8200"
	defaultAgentBufferSize      int           = 100
)

// Client is the client used to communicate with the apm server.
type Client struct {
	mu                   sync.Mutex
	bufferPool           sync.Pool
	AgentDoneSignal      chan struct{}
	DataChannel          chan AgentData
	client               *http.Client
	Status               Status
	ReconnectionCount    int
	ServerAPIKey         string
	ServerSecretToken    string
	serverURL            string
	dataForwarderTimeout time.Duration
	receiver             *http.Server
}

func NewClient(opts ...Option) (*Client, error) {
	c := Client{
		bufferPool: sync.Pool{New: func() interface{} {
			return &bytes.Buffer{}
		}},
		DataChannel: make(chan AgentData, defaultAgentBufferSize),
		client: &http.Client{
			Transport: http.DefaultTransport.(*http.Transport).Clone(),
		},
		dataForwarderTimeout: defaultDataForwarderTimeout,
		ReconnectionCount:    -1,
		Status:               Healthy,
		receiver: &http.Server{
			Addr:           defaultReceiverAddr,
			ReadTimeout:    defaultDataReceiverTimeout,
			WriteTimeout:   defaultDataReceiverTimeout,
			MaxHeaderBytes: 1 << 20,
		},
	}

	for _, opt := range opts {
		opt(&c)
	}

	if c.serverURL == "" {
		return nil, errors.New("APM Server URL cannot be empty")
	}

	// normalize server URL
	if !strings.HasSuffix(c.serverURL, "/") {
		c.serverURL = c.serverURL + "/"
	}

	rand.Seed(time.Now().UnixNano())

	return &c, nil
}
