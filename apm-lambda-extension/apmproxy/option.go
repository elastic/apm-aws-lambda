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
	"time"

	"go.uber.org/zap"
)

type Option func(*Client)

func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.client.Timeout = timeout
	}
}

func WithAPIKey(key string) Option {
	return func(c *Client) {
		c.ServerAPIKey = key
	}
}

func WithSecretToken(secret string) Option {
	return func(c *Client) {
		c.ServerSecretToken = secret
	}
}

func WithURL(url string) Option {
	return func(c *Client) {
		c.serverURL = url
	}
}

func WithDataForwarderTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.dataForwarderTimeout = timeout
	}
}

// WithReceiverTimeout sets the timeout receiver.
func WithReceiverTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.receiver.ReadTimeout = timeout
		c.receiver.WriteTimeout = timeout
	}
}

// WithReceiverAddress sets the receiver address.
func WithReceiverAddress(addr string) Option {
	return func(c *Client) {
		c.receiver.Addr = addr
	}
}

// WithAgentDataBufferSize sets the agent data buffer size.
func WithAgentDataBufferSize(size int) Option {
	return func(c *Client) {
		c.DataChannel = make(chan AgentData, size)
	}
}

func WithLogger(logger *zap.SugaredLogger) Option {
	return func(c *Client) {
		c.logger = logger
	}
}
