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
	"bytes"
	"compress/gzip"
	"context"
	"elastic/apm-lambda-extension/extension"
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
			extension.Log.Debug("Invocation context cancelled, not processing any more agent data")
			return nil
		case agentData := <-c.DataChannel:
			if metadataContainer.Metadata == nil {
				metadata, err := ProcessMetadata(agentData)
				if err != nil {
					extension.Log.Errorf("Error extracting metadata from agent payload %v", err)
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
		extension.Log.Debug("Flush skipped - Transport failing")
		return
	}
	extension.Log.Debug("Flush started - Checking for agent data")
	for {
		select {
		case agentData := <-c.DataChannel:
			extension.Log.Debug("Flush in progress - Processing agent data")
			if err := c.PostToApmServer(ctx, agentData); err != nil {
				extension.Log.Errorf("Error sending to APM server, skipping: %v", err)
			}
		default:
			extension.Log.Debug("Flush ended - No agent data on buffer")
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
			extension.Log.Errorf("Failed to compress data: %v", err)
		}
		if err := gw.Close(); err != nil {
			extension.Log.Errorf("Failed write compressed data to buffer: %v", err)
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

	extension.Log.Debug("Sending data chunk to APM server")
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
		extension.Log.Warnf("Authentication with the APM server failed: response status code: %d", resp.StatusCode)
		extension.Log.Debugf("APM server response body: %v", string(body))
		return nil
	}

	c.SetApmServerTransportState(ctx, Healthy)
	extension.Log.Debug("Transport status set to healthy")
	extension.Log.Debugf("APM server response body: %v", string(body))
	extension.Log.Debugf("APM server response status code: %v", resp.StatusCode)
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
		extension.Log.Debugf("APM server Transport status set to %s", c.Status)
		c.ReconnectionCount = -1
		c.mu.Unlock()
	case Failing:
		c.mu.Lock()
		c.Status = status
		extension.Log.Debugf("APM server Transport status set to %s", c.Status)
		c.ReconnectionCount++
		gracePeriodTimer := time.NewTimer(c.ComputeGracePeriod())
		extension.Log.Debugf("Grace period entered, reconnection count : %d", c.ReconnectionCount)
		go func() {
			select {
			case <-gracePeriodTimer.C:
				extension.Log.Debug("Grace period over - timer timed out")
			case <-ctx.Done():
				extension.Log.Debug("Grace period over - context done")
			}
			c.Status = Pending
			extension.Log.Debugf("APM server Transport status set to %s", c.Status)
			c.mu.Unlock()
		}()
	default:
		extension.Log.Errorf("Cannot set APM server Transport status to %s", status)
	}
}

// ComputeGracePeriod https://github.com/elastic/apm/blob/main/specs/agents/transport.md#transport-errors
func (c *Client) ComputeGracePeriod() time.Duration {
	gracePeriodWithoutJitter := math.Pow(math.Min(float64(c.ReconnectionCount), 6), 2)
	jitter := rand.Float64()/5 - 0.1
	return time.Duration((gracePeriodWithoutJitter + jitter*gracePeriodWithoutJitter) * float64(time.Second))
}

// EnqueueAPMData adds a AgentData struct to the agent data channel, effectively queueing for a send
// to the APM server.
func (c *Client) EnqueueAPMData(agentData AgentData) {
	select {
	case c.DataChannel <- agentData:
		extension.Log.Debug("Adding agent data to buffer to be sent to apm server")
	default:
		extension.Log.Warn("Channel full: dropping a subset of agent data")
	}
}
