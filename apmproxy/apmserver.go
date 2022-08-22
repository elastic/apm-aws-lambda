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
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"time"
)

// ForwardApmData receives agent data as it comes in and posts it to the APM server.
// Stop checking for, and sending agent data when the function invocation
// has completed, signaled via a channel.
func (c *Client) ForwardApmData(ctx context.Context, metadataContainer *MetadataContainer) error {
	if c.Status == Failing {
		return nil
	}
	for {
		select {
		case <-ctx.Done():
			c.logger.Debug("Invocation context cancelled, not processing any more agent data")
			return nil
		case agentData := <-c.DataChannel:
			if metadataContainer.Metadata == nil {
				metadata, err := ProcessMetadata(agentData)
				if err != nil {
					return fmt.Errorf("failed to extract metadata from agent payload %w", err)
				}
				metadataContainer.Metadata = metadata
			}
			if err := c.PostToApmServer(ctx, agentData); err != nil {
				return fmt.Errorf("error sending to APM server, skipping: %v", err)
			}
		}
	}
}

// FlushAPMData reads all the apm data in the apm data channel and sends it to the APM server.
func (c *Client) FlushAPMData(ctx context.Context) {
	if c.Status == Failing {
		c.logger.Debug("Flush skipped - Transport failing")
		return
	}
	c.logger.Debug("Flush started - Checking for agent data")
	for {
		select {
		case agentData := <-c.DataChannel:
			c.logger.Debug("Flush in progress - Processing agent data")
			if err := c.PostToApmServer(ctx, agentData); err != nil {
				c.logger.Errorf("Error sending to APM server, skipping: %v", err)
			}
		default:
			c.logger.Debug("Flush ended - No agent data on buffer")
			return
		}
	}
}

// PostToApmServer takes a chunk of APM agent data and posts it to the APM server.
//
// The function compresses the APM agent data, if it's not already compressed.
// It sets the APM transport status to failing upon errors, as part of the backoff
// strategy.
func (c *Client) PostToApmServer(ctx context.Context, agentData AgentData) error {
	// todo: can this be a streaming or streaming style call that keeps the
	//       connection open across invocations?
	if c.Status == Failing {
		return errors.New("transport status is unhealthy")
	}

	endpointURI := "intake/v2/events"
	encoding := agentData.ContentEncoding

	var r io.Reader
	if agentData.ContentEncoding != "" {
		r = bytes.NewReader(agentData.Data)
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
		if _, err := gw.Write(agentData.Data); err != nil {
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
	if c.ServerAPIKey != "" {
		req.Header.Add("Authorization", "ApiKey "+c.ServerAPIKey)
	} else if c.ServerSecretToken != "" {
		req.Header.Add("Authorization", "Bearer "+c.ServerSecretToken)
	}

	c.logger.Debug("Sending data chunk to APM server")
	resp, err := c.client.Do(req)
	if err != nil {
		c.SetApmServerTransportState(ctx, Failing)
		return fmt.Errorf("failed to post to APM server: %v", err)
	}

	//Read the response body
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.SetApmServerTransportState(ctx, Failing)
		return fmt.Errorf("failed to read the response body after posting to the APM server")
	}

	if resp.StatusCode == http.StatusUnauthorized {
		c.logger.Warnf("Authentication with the APM server failed: response status code: %d", resp.StatusCode)
		c.logger.Debugf("APM server response body: %v", string(body))
		return nil
	}

	c.SetApmServerTransportState(ctx, Healthy)
	c.logger.Debug("Transport status set to healthy")
	c.logger.Debugf("APM server response body: %v", string(body))
	c.logger.Debugf("APM server response status code: %v", resp.StatusCode)
	return nil
}

// SetApmServerTransportState takes a state of the APM server transport and updates
// the current state of the transport. For a change to a failing state, the grace period
// is calculated and a go routine is started that waits for that period to complete
// before changing the status to "pending". This would allow a subsequent send attempt
// to the APM server.
//
// This function is public for use in tests.
func (c *Client) SetApmServerTransportState(ctx context.Context, status Status) {
	switch status {
	case Healthy:
		c.mu.Lock()
		c.Status = status
		c.logger.Debugf("APM server Transport status set to %s", c.Status)
		c.ReconnectionCount = -1
		c.mu.Unlock()
	case Failing:
		c.mu.Lock()
		c.Status = status
		c.logger.Debugf("APM server Transport status set to %s", c.Status)
		c.ReconnectionCount++
		gracePeriodTimer := time.NewTimer(c.ComputeGracePeriod())
		c.logger.Debugf("Grace period entered, reconnection count : %d", c.ReconnectionCount)
		go func() {
			select {
			case <-gracePeriodTimer.C:
				c.logger.Debug("Grace period over - timer timed out")
			case <-ctx.Done():
				c.logger.Debug("Grace period over - context done")
			}
			c.Status = Pending
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
		gracePeriod := rand.Float64() * 5
		return time.Duration(gracePeriod * float64(time.Second))
	}
	gracePeriodWithoutJitter := math.Pow(math.Min(float64(c.ReconnectionCount), 6), 2)
	jitter := rand.Float64()/5 - 0.1
	return time.Duration((gracePeriodWithoutJitter + jitter*gracePeriodWithoutJitter) * float64(time.Second))
}

// EnqueueAPMData adds a AgentData struct to the agent data channel, effectively queueing for a send
// to the APM server.
func (c *Client) EnqueueAPMData(agentData AgentData) {
	select {
	case c.DataChannel <- agentData:
		c.logger.Debug("Adding agent data to buffer to be sent to apm server")
	default:
		c.logger.Warn("Channel full: dropping a subset of agent data")
	}
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

// WaitForFlush returns a channel that is closed when the agent has signalled that
// the Lambda invocation has completed, and there is no more APM data coming.
func (c *Client) WaitForFlush() <-chan struct{} {
	c.flushMutex.Lock()
	defer c.flushMutex.Unlock()
	return c.flushCh
}
