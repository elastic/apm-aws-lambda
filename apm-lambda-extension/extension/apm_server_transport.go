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
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// Constants for the state of the transport used in
// the backoff implementation.
type ApmServerTransportStatusType string

const (
	Failing ApmServerTransportStatusType = "Failing"
	Pending ApmServerTransportStatusType = "Pending"
	Healthy ApmServerTransportStatusType = "Healthy"
)

// A struct to track the state and status of sending
// to the APM server. Used in the backoff implementation.
type ApmServerTransport struct {
	sync.Mutex
	bufferPool        sync.Pool
	config            *extensionConfig
	AgentDoneSignal   chan struct{}
	dataChannel       chan AgentData
	client            *http.Client
	status            ApmServerTransportStatusType
	reconnectionCount int
	gracePeriodTimer  *time.Timer
}

func InitApmServerTransport(config *extensionConfig) *ApmServerTransport {
	var transport ApmServerTransport
	transport.bufferPool = sync.Pool{New: func() interface{} {
		return &bytes.Buffer{}
	}}
	transport.dataChannel = make(chan AgentData, 100)
	transport.client = &http.Client{
		Timeout:   time.Duration(config.DataForwarderTimeoutSeconds) * time.Second,
		Transport: http.DefaultTransport.(*http.Transport).Clone(),
	}
	transport.config = config
	transport.status = Healthy
	transport.reconnectionCount = -1
	return &transport
}

// StartBackgroundApmDataForwarding Receive agent data as it comes in and post it to the APM server.
// Stop checking for, and sending agent data when the function invocation
// has completed, signaled via a channel.
func (transport *ApmServerTransport) ForwardApmData(ctx context.Context, metadataContainer *MetadataContainer) error {
	if transport.status == Failing {
		return nil
	}
	for {
		select {
		case <-ctx.Done():
			Log.Debug("Invocation context cancelled, not processing any more agent data")
			return nil
		case agentData := <-transport.dataChannel:
			if metadataContainer.Metadata == nil {
				metadata, err := ProcessMetadata(agentData)
				if err != nil {
					Log.Errorf("Error extracting metadata from agent payload %v", err)
				}
				metadataContainer.Metadata = metadata
			}
			if err := transport.PostToApmServer(ctx, agentData); err != nil {
				return fmt.Errorf("error sending to APM server, skipping: %v", err)
			}
		}
	}
}

// FlushAPMData reads all the apm data in the apm data channel and sends it to the APM server.
func (transport *ApmServerTransport) FlushAPMData(ctx context.Context) {
	if transport.status == Failing {
		Log.Debug("Flush skipped - Transport failing")
		return
	}
	Log.Debug("Flush started - Checking for agent data")
	for {
		select {
		case agentData := <-transport.dataChannel:
			Log.Debug("Flush in progress - Processing agent data")
			if err := transport.PostToApmServer(ctx, agentData); err != nil {
				Log.Errorf("Error sending to APM server, skipping: %v", err)
			}
		default:
			Log.Debug("Flush ended - No agent data on buffer")
			return
		}
	}
}

// PostToApmServer takes a chunk of APM agent data and posts it to the APM server.
//
// The function compresses the APM agent data, if it's not already compressed.
// It sets the APM transport status to failing upon errors, as part of the backoff
// strategy.
func (transport *ApmServerTransport) PostToApmServer(ctx context.Context, agentData AgentData) error {
	// todo: can this be a streaming or streaming style call that keeps the
	//       connection open across invocations?
	if transport.status == Failing {
		return errors.New("transport status is unhealthy")
	}

	endpointURI := "intake/v2/events"
	encoding := agentData.ContentEncoding

	var r io.Reader
	if agentData.ContentEncoding != "" {
		r = bytes.NewReader(agentData.Data)
	} else {
		encoding = "gzip"
		buf := transport.bufferPool.Get().(*bytes.Buffer)
		defer func() {
			buf.Reset()
			transport.bufferPool.Put(buf)
		}()
		gw, err := gzip.NewWriterLevel(buf, gzip.BestSpeed)
		if err != nil {
			return err
		}
		if _, err := gw.Write(agentData.Data); err != nil {
			Log.Errorf("Failed to compress data: %v", err)
		}
		if err := gw.Close(); err != nil {
			Log.Errorf("Failed write compressed data to buffer: %v", err)
		}
		r = buf
	}

	req, err := http.NewRequest("POST", transport.config.apmServerUrl+endpointURI, r)
	if err != nil {
		return fmt.Errorf("failed to create a new request when posting to APM server: %v", err)
	}
	req.Header.Add("Content-Encoding", encoding)
	req.Header.Add("Content-Type", "application/x-ndjson")
	if transport.config.apmServerApiKey != "" {
		req.Header.Add("Authorization", "ApiKey "+transport.config.apmServerApiKey)
	} else if transport.config.apmServerSecretToken != "" {
		req.Header.Add("Authorization", "Bearer "+transport.config.apmServerSecretToken)
	}

	Log.Debug("Sending data chunk to APM server")
	resp, err := transport.client.Do(req)
	if err != nil {
		transport.SetApmServerTransportState(ctx, Failing)
		return fmt.Errorf("failed to post to APM server: %v", err)
	}

	//Read the response body
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		transport.SetApmServerTransportState(ctx, Failing)
		return fmt.Errorf("failed to read the response body after posting to the APM server")
	}

	// On success, the server will respond with a 202 Accepted status code.
	// Log a warning otherwise.
	if resp.StatusCode != http.StatusAccepted {
		Log.Warnf("APM server request failed with status code: %d", resp.StatusCode)
	}

	transport.SetApmServerTransportState(ctx, Healthy)
	Log.Debug("Transport status set to healthy")
	Log.Debugf("APM server response body: %v", string(body))
	Log.Debugf("APM server response status code: %v", resp.StatusCode)
	return nil
}

// SetApmServerTransportState takes a state of the APM server transport and updates
// the current state of the transport. For a change to a failing state, the grace period
// is calculated and a go routine is started that waits for that period to complete
// before changing the status to "pending". This would allow a subsequent send attempt
// to the APM server.
//
// This function is public for use in tests.
func (transport *ApmServerTransport) SetApmServerTransportState(ctx context.Context, status ApmServerTransportStatusType) {
	switch status {
	case Healthy:
		transport.Lock()
		transport.status = status
		Log.Debugf("APM server Transport status set to %s", transport.status)
		transport.reconnectionCount = -1
		transport.Unlock()
	case Failing:
		transport.Lock()
		transport.status = status
		Log.Debugf("APM server Transport status set to %s", transport.status)
		transport.reconnectionCount++
		transport.gracePeriodTimer = time.NewTimer(transport.computeGracePeriod())
		Log.Debugf("Grace period entered, reconnection count : %d", transport.reconnectionCount)
		go func() {
			select {
			case <-transport.gracePeriodTimer.C:
				Log.Debug("Grace period over - timer timed out")
			case <-ctx.Done():
				Log.Debug("Grace period over - context done")
			}
			transport.status = Pending
			Log.Debugf("APM server Transport status set to %s", transport.status)
			transport.Unlock()
		}()
	default:
		Log.Errorf("Cannot set APM server Transport status to %s", status)
	}
}

// ComputeGracePeriod https://github.com/elastic/apm/blob/main/specs/agents/transport.md#transport-errors
func (transport *ApmServerTransport) computeGracePeriod() time.Duration {
	gracePeriodWithoutJitter := math.Pow(math.Min(float64(transport.reconnectionCount), 6), 2)
	jitter := rand.Float64()/5 - 0.1
	return time.Duration((gracePeriodWithoutJitter + jitter*gracePeriodWithoutJitter) * float64(time.Second))
}

// EnqueueAPMData adds a AgentData struct to the agent data channel, effectively queueing for a send
// to the APM server.
func (transport *ApmServerTransport) EnqueueAPMData(agentData AgentData) {
	select {
	case transport.dataChannel <- agentData:
		Log.Debug("Adding agent data to buffer to be sent to apm server")
	default:
		Log.Warn("Channel full: dropping a subset of agent data")
	}
}
