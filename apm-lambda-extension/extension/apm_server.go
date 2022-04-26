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

var bufferPool = sync.Pool{New: func() interface{} {
	return &bytes.Buffer{}
}}

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
	sync.Pool
	sync.Mutex
	ctx               context.Context
	config            *extensionConfig
	DataChannel       chan AgentData
	Client            *http.Client
	Status            ApmServerTransportStatusType
	ReconnectionCount int
	GracePeriodTimer  *time.Timer
}

func InitApmServerTransport(ctx context.Context, config *extensionConfig) *ApmServerTransport {
	var transport ApmServerTransport
	transport.DataChannel = make(chan AgentData, 100)
	transport.Client = &http.Client{
		Timeout:   time.Duration(config.DataForwarderTimeoutSeconds) * time.Second,
		Transport: http.DefaultTransport.(*http.Transport).Clone(),
	}
	transport.config = config
	transport.ctx = ctx
	transport.Status = Healthy
	transport.ReconnectionCount = -1
	return &transport
}

func StartBackgroundSending(transport *ApmServerTransport, funcDone chan struct{}, backgroundDataSendWg *sync.WaitGroup) {
	go func() {
		defer backgroundDataSendWg.Done()
		if transport.Status == Failing {
			return
		}
		for {
			select {
			case <-funcDone:
				Log.Debug("Received signal that function has completed, not processing any more agent data")
				return
			case agentData := <-transport.DataChannel:
				if err := PostToApmServer(transport, agentData); err != nil {
					Log.Errorf("Error sending to APM server, skipping: %v", err)
					return
				}
			}
		}
	}()
}

// PostToApmServer takes a chunk of APM agent data and posts it to the APM server.
//
// The function compresses the APM agent data, if it's not already compressed.
// It sets the APM transport status to failing upon errors, as part of the backoff
// strategy.
func PostToApmServer(transport *ApmServerTransport, agentData AgentData) error {
	// todo: can this be a streaming or streaming style call that keeps the
	//       connection open across invocations?
	if transport.Status == Failing {
		return errors.New("transport status is unhealthy")
	}

	endpointURI := "intake/v2/events"
	encoding := agentData.ContentEncoding

	var r io.Reader
	if agentData.ContentEncoding != "" {
		r = bytes.NewReader(agentData.Data)
	} else {
		encoding = "gzip"
		buf := bufferPool.Get().(*bytes.Buffer)
		defer func() {
			buf.Reset()
			bufferPool.Put(buf)
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

	Log.Debug("Sending data chunk to APM Server")
	resp, err := transport.Client.Do(req)
	if err != nil {
		SetApmServerTransportState(transport, Failing)
		return fmt.Errorf("failed to post to APM server: %v", err)
	}

	//Read the response body
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		SetApmServerTransportState(transport, Failing)
		return fmt.Errorf("failed to read the response body after posting to the APM server")
	}

	SetApmServerTransportState(transport, Healthy)
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
func SetApmServerTransportState(transport *ApmServerTransport, status ApmServerTransportStatusType) {
	switch status {
	case Healthy:
		transport.Lock()
		transport.Status = status
		Log.Debugf("APM Server Transport status set to %s", transport.Status)
		transport.ReconnectionCount = -1
		transport.Unlock()
	case Failing:
		transport.Lock()
		transport.Status = status
		Log.Debugf("APM Server Transport status set to %s", transport.Status)
		transport.ReconnectionCount++
		transport.GracePeriodTimer = time.NewTimer(computeGracePeriod(transport))
		Log.Debugf("Grace period entered, reconnection count : %d", transport.ReconnectionCount)
		go func() {
			select {
			case <-transport.GracePeriodTimer.C:
				Log.Debug("Grace period over - timer timed out")
			case <-transport.ctx.Done():
				Log.Debug("Grace period over - context done")
			}
			transport.Status = Pending
			Log.Debugf("APM Server Transport status set to %s", transport.Status)
			transport.Unlock()
		}()
	default:
		Log.Errorf("Cannot set APM Server Transport status to %s", status)
	}
}

// ComputeGracePeriod https://github.com/elastic/apm/blob/main/specs/agents/transport.md#transport-errors
func computeGracePeriod(transport *ApmServerTransport) time.Duration {
	gracePeriodWithoutJitter := math.Pow(math.Min(float64(transport.ReconnectionCount), 6), 2)
	jitter := rand.Float64()/5 - 0.1
	return time.Duration((gracePeriodWithoutJitter + jitter*gracePeriodWithoutJitter) * float64(time.Second))
}

// EnqueueAPMData adds a AgentData struct to the agent data channel, effectively queueing for a send
// to the APM server.
func EnqueueAPMData(agentDataChannel chan AgentData, agentData AgentData) {
	select {
	case agentDataChannel <- agentData:
		Log.Debug("Adding agent data to buffer to be sent to apm server")
	default:
		Log.Warn("Channel full: dropping a subset of agent data")
	}
}
