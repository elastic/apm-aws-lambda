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

import "go.uber.org/zap"

// WithListenerAddress sets the listener address of the
// server listening for logs event.
func WithListenerAddress(s string) ClientOption {
	return func(c *Client) {
		c.listenerAddr = s
	}
}

// WithLogsAPIBaseURL sets the logs api base url.
func WithLogsAPIBaseURL(s string) ClientOption {
	return func(c *Client) {
		c.logsAPIBaseURL = s
	}
}

// WithLogBuffer sets the size of the buffer
// storing queued logs for processing.
func WithLogBuffer(size int) ClientOption {
	return func(c *Client) {
		c.logsChannel = make(chan LogEvent, size)
	}
}

// WithLogger sets the logger.
func WithLogger(logger *zap.SugaredLogger) ClientOption {
	return func(c *Client) {
		c.logger = logger
	}
}

// WithLogsAPISubscriptionTypes sets the logstreams that the Logs API will subscribe to.
func WithLogsAPISubscriptionTypes(types ...SubscriptionType) ClientOption {
	return func(c *Client) {
		c.logsAPISubscriptionTypes = types
	}
}
