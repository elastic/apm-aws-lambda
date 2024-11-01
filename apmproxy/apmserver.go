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
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"time"

	"github.com/elastic/apm-aws-lambda/accumulator"
	"github.com/elastic/apm-aws-lambda/version"
	"go.uber.org/zap"
)

type jsonResult struct {
	Errors []jsonError `json:"errors,omitempty"`
}

type jsonError struct {
	Message  string `json:"message"`
	Document string `json:"document,omitempty"`
}

// ForwardApmData receives apm data as it comes in and posts it to the APM server.
// Stop checking for, and sending apm data when the function invocation
// has completed, signaled via a channel.
func (c *Client) ForwardApmData(ctx context.Context) error {
	if c.IsUnhealthy() {
		c.logger.Warn("Failed to start APM data forwarder due to client unhealthy")
		return nil
	}
	var lambdaDataChan chan []byte
	for {
		select {
		case <-ctx.Done():
			c.logger.Debug("Invocation context canceled, not processing any more agent data")
			return nil
		case data := <-c.AgentDataChannel:
			if len(data.Data) == 0 {
				c.logger.Debugf("Received something from '%s' without APMData", data.AgentInfo)
				continue
			}
			if err := c.forwardAgentData(ctx, data); err != nil {
				return err
			}
			if lambdaDataChan == nil {
				// With the first successful request to c.forwardAgent Data() metadata should be
				// available and processing data from c.LambdaDataChannel can start.
				lambdaDataChan = c.LambdaDataChannel
				c.logger.Debug("Assigned Lambda data channel")
			}
		case data := <-lambdaDataChan:
			if err := c.forwardLambdaData(ctx, data); err != nil {
				return err
			}
		}
	}
}

// FlushAPMData reads all the apm data in the apm data channel and sends it to the APM server.
func (c *Client) FlushAPMData(ctx context.Context) {
	if c.IsUnhealthy() {
		c.logger.Debug("Flush skipped - Transport failing")
		return
	}
	c.logger.Debug("Flush started - Checking for agent data")

	// Flush agent data first to make sure metadata is available if possible
	for i := len(c.AgentDataChannel); i > 0; i-- {
		data := <-c.AgentDataChannel
		if err := c.forwardAgentData(ctx, data); err != nil {
			c.logger.Errorf("Error sending to APM Server, skipping: %v", err)
		}
	}

	// If metadata still not available then fail fast
	if c.batch == nil {
		c.logger.Warnf("Metadata not available at flush, skipping sending lambda data to APM Server")
		return
	}

	// Flush lambda data
	c.logger.Debug("Flush in progress - Processing lambda data")
	for {
		select {
		case apmData := <-c.LambdaDataChannel:
			if err := c.forwardLambdaData(ctx, apmData); err != nil {
				c.logger.Errorf("Error sending to APM server, skipping: %v", err)
			}
		case <-ctx.Done():
			c.logger.Debug("Failed to flush completely, may result in data drop")
			return
		default:
			// Flush any remaining data in batch
			if err := c.sendBatch(ctx); err != nil {
				c.logger.Errorf("Error sending to APM server, skipping: %v", err)
			}
			c.logger.Debug("Flush ended for lambda data - no data in buffer")
			return
		}
	}
}

// PostToApmServer takes a chunk of APM agent data and posts it to the APM server.
//
// The function compresses the APM agent data, if it's not already compressed.
// It sets the APM transport status to failing upon errors, as part of the backoff
// strategy.
func (c *Client) PostToApmServer(ctx context.Context, apmData accumulator.APMData) error {
	// todo: can this be a streaming or streaming style call that keeps the
	//       connection open across invocations?
	if c.IsUnhealthy() {
		return errors.New("transport status is unhealthy")
	}

	endpointURI := "intake/v2/events"
	encoding := apmData.ContentEncoding
	agentInfo := apmData.AgentInfo

	var r io.Reader
	if apmData.ContentEncoding != "" {
		r = bytes.NewReader(apmData.Data)
	} else {
		encoding = "gzip"
		buf := c.bufferPool.Get().(*bytes.Buffer)
		defer func() {
			buf.Reset()
			c.bufferPool.Put(buf)
		}()
		gw, err := gzip.NewWriterLevel(buf, gzip.BestSpeed)
		if err != nil {
			return err
		}
		if _, err := gw.Write(apmData.Data); err != nil {
			return fmt.Errorf("failed to compress data: %w", err)
		}
		if err := gw.Close(); err != nil {
			return fmt.Errorf("failed to write compressed data to buffer: %w", err)
		}
		r = buf
	}

	req, err := http.NewRequest(http.MethodPost, c.serverURL+endpointURI, r)
	if err != nil {
		return fmt.Errorf("failed to create a new request when posting to APM server: %v", err)
	}
	req.Header.Add("Content-Encoding", encoding)
	req.Header.Add("Content-Type", "application/x-ndjson")
	req.Header.Set("User-Agent", version.UserAgent+" "+agentInfo)
	if c.ServerAPIKey != "" {
		req.Header.Add("Authorization", "ApiKey "+c.ServerAPIKey)
	} else if c.ServerSecretToken != "" {
		req.Header.Add("Authorization", "Bearer "+c.ServerSecretToken)
	}

	c.logger.Debug("Sending data chunk to APM server")
	resp, err := c.client.Do(req)
	if err != nil {
		c.UpdateStatus(ctx, Failing)
		return fmt.Errorf("failed to post to APM server: %v", err)
	}
	defer resp.Body.Close()

	// On success, the server will respond with a 202 Accepted status code and no body.
	if resp.StatusCode == http.StatusAccepted {
		c.UpdateStatus(ctx, Healthy)
		return nil
	}

	// RateLimited
	if resp.StatusCode == http.StatusTooManyRequests {
		c.logger.Warnf("Transport has been rate limited: response status code: %d", resp.StatusCode)
		c.UpdateStatus(ctx, RateLimited)
		return nil
	}

	// Auth errors
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		logBodyErrors(c.logger, resp)
		c.UpdateStatus(ctx, Failing)
		return nil
	}

	// ClientErrors
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		logBodyErrors(c.logger, resp)
		c.UpdateStatus(ctx, ClientFailing)
		return nil
	}

	// critical errors
	if resp.StatusCode == http.StatusInternalServerError || resp.StatusCode == http.StatusServiceUnavailable {
		logBodyErrors(c.logger, resp)
		c.UpdateStatus(ctx, Failing)
		return nil
	}

	c.logger.Warnf("unhandled status code: %d", resp.StatusCode)
	return nil
}

func logBodyErrors(logger *zap.SugaredLogger, resp *http.Response) {
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Warnf("failed to post data to APM server: response status: %s: failed to read response body: %v", resp.Status, err)
		return
	}

	jErr := jsonResult{}
	if err := json.Unmarshal(b, &jErr); err != nil {
		logger.Warnf("failed to post data to APM server: response status: %s: failed to decode response body: %v: body: %s", resp.Status, err, string(b))
		return
	}

	if len(jErr.Errors) == 0 {
		logger.Warnf("failed to post data to APM server: response status: %s: response body: %s", resp.Status, string(b))
		return
	}

	logger.Warnf("failed to post data to APM server: response status: %s", resp.Status)
	for _, err := range jErr.Errors {
		logger.Warnf("document %s: message: %s", err.Document, err.Message)
	}
}

// IsUnhealthy returns true if the apmproxy is not healthy.
func (c *Client) IsUnhealthy() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Status == Failing
}

// UpdateStatus takes a state of the APM server transport and updates
// the current state of the transport. For a change to a failing state, the grace period
// is calculated and a go routine is started that waits for that period to complete
// before changing the status to "pending". This would allow a subsequent send attempt
// to the APM server.
//
// This function is public for use in tests.
func (c *Client) UpdateStatus(ctx context.Context, status Status) {
	// Reduce lock contention as UpdateStatus is called on every
	// successful request
	c.mu.RLock()
	if status == c.Status {
		c.mu.RUnlock()
		return
	}
	c.mu.RUnlock()

	switch status {
	case Healthy:
		c.mu.Lock()
		if c.Status == status {
			return
		}
		c.Status = status
		c.logger.Debugf("APM server Transport status set to %s", c.Status)
		c.ReconnectionCount = -1
		c.mu.Unlock()
	case RateLimited, ClientFailing:
		// No need to start backoff, this is a temporary status. It usually
		// means we went over the limit of events/s.
		c.mu.Lock()
		c.Status = status
		c.logger.Debugf("APM server Transport status set to %s", c.Status)
		c.mu.Unlock()
	case Failing:
		c.mu.Lock()
		c.Status = status
		c.logger.Debugf("APM server Transport status set to %s", c.Status)
		c.ReconnectionCount++
		gracePeriodTimer := time.NewTimer(c.ComputeGracePeriod())
		c.logger.Debugf("Grace period entered, reconnection count : %d", c.ReconnectionCount)
		c.mu.Unlock()

		go func() {
			select {
			case <-gracePeriodTimer.C:
				c.logger.Debug("Grace period over - timer timed out")
			case <-ctx.Done():
				c.logger.Debug("Grace period over - context done")
			}
			c.mu.Lock()
			c.Status = Started
			c.logger.Debugf("APM server Transport status set to %s", c.Status)
			c.mu.Unlock()
		}()
	default:
		c.logger.Errorf("Cannot set APM server Transport status to %s", status)
	}
}

// ComputeGracePeriod https://github.com/elastic/apm/blob/main/specs/agents/transport.md#transport-errors
func (c *Client) ComputeGracePeriod() time.Duration {
	// If reconnectionCount is 0, returns a random number in an interval.
	// The grace period for the first reconnection count was 0 but that
	// leads to collisions with multiple environments.
	if c.ReconnectionCount == 0 {
		gracePeriod := rand.Float64() * 5 //nolint:gosec
		return time.Duration(gracePeriod * float64(time.Second))
	}
	gracePeriodWithoutJitter := math.Pow(math.Min(float64(c.ReconnectionCount), 6), 2)
	jitter := rand.Float64()/5 - 0.1 //nolint:gosec
	return time.Duration((gracePeriodWithoutJitter + jitter*gracePeriodWithoutJitter) * float64(time.Second))
}

// ShouldFlush returns true if the client should flush APM data after processing the event.
func (c *Client) ShouldFlush() bool {
	return c.sendStrategy == SyncFlush
}

// ResetFlush resets the client's "agent flushed" state, such that
// subsequent calls to WaitForFlush will block until another request
// is received from the agent indicating it has flushed.
func (c *Client) ResetFlush() {
	c.flushMutex.Lock()
	defer c.flushMutex.Unlock()
	c.flushCh = make(chan struct{})
}

// WaitForFlush returns a channel that is closed when the agent has signaled that
// the Lambda invocation has completed, and there is no more APM data coming.
func (c *Client) WaitForFlush() <-chan struct{} {
	c.flushMutex.Lock()
	defer c.flushMutex.Unlock()
	return c.flushCh
}

func (c *Client) forwardAgentData(ctx context.Context, apmData accumulator.APMData) error {
	if err := c.batch.AddAgentData(apmData); err != nil {
		c.logger.Warnf("Dropping agent data due to error: %v", err)
	}
	if c.batch.ShouldShip() {
		return c.sendBatch(ctx)
	}
	return nil
}

func (c *Client) forwardLambdaData(ctx context.Context, data []byte) error {
	if err := c.batch.AddLambdaData(data); err != nil {
		c.logger.Warnf("Dropping lambda data due to error: %v", err)
	}
	if c.batch.ShouldShip() {
		return c.sendBatch(ctx)
	}
	return nil
}

func (c *Client) sendBatch(ctx context.Context) error {
	if c.batch == nil || c.batch.Count() == 0 {
		return nil
	}
	defer c.batch.Reset()
	return c.PostToApmServer(ctx, c.batch.ToAPMData())
}
