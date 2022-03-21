// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
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

type ApmServerTransportStatusType string

const (
	Failing ApmServerTransportStatusType = "Failing"
	Pending ApmServerTransportStatusType = "Pending"
	Healthy ApmServerTransportStatusType = "Healthy"
)

type ApmServerTransportState struct {
	sync.Mutex
	Status            ApmServerTransportStatusType
	ReconnectionCount int
	GracePeriodTimer  *time.Timer
}

var apmServerTransportState = ApmServerTransportState{
	Status:            Healthy,
	ReconnectionCount: 0,
}

func SetApmServerTransportStatus(status ApmServerTransportStatusType, reconnectionCount int) {
	apmServerTransportState.Status = status
	apmServerTransportState.ReconnectionCount = reconnectionCount
}

// todo: can this be a streaming or streaming style call that keeps the
//       connection open across invocations?
func PostToApmServer(client *http.Client, agentData AgentData, config *extensionConfig, ctx context.Context) error {
	if !IsTransportStatusHealthyOrPending() {
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

	req, err := http.NewRequest("POST", config.apmServerUrl+endpointURI, r)
	if err != nil {
		return fmt.Errorf("failed to create a new request when posting to APM server: %v", err)
	}
	req.Header.Add("Content-Encoding", encoding)
	req.Header.Add("Content-Type", "application/x-ndjson")
	if config.apmServerApiKey != "" {
		req.Header.Add("Authorization", "ApiKey "+config.apmServerApiKey)
	} else if config.apmServerSecretToken != "" {
		req.Header.Add("Authorization", "Bearer "+config.apmServerSecretToken)
	}

	Log.Debug("Sending data chunk to APM Server")
	resp, err := client.Do(req)
	if err != nil {
		enterBackoff(ctx)
		return fmt.Errorf("failed to post to APM server: %v", err)
	}

	//Read the response body
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		enterBackoff(ctx)
		return fmt.Errorf("failed to read the response body after posting to the APM server")
	}

	apmServerTransportState.Status = Healthy
	Log.Debug("Transport status set to healthy")
	Log.Debugf("APM server response body: %v", string(body))
	Log.Debugf("APM server response status code: %v", resp.StatusCode)
	return nil
}

func EnqueueAPMData(agentDataChannel chan AgentData, agentData AgentData) {
	select {
	case agentDataChannel <- agentData:
		Log.Debug("Adding agent data to buffer to be sent to apm server")
	default:
		Log.Warn("Channel full: dropping a subset of agent data")
	}
}

func IsTransportStatusHealthyOrPending() bool {
	return apmServerTransportState.Status != Failing
}

func WaitForGracePeriod(ctx context.Context) {
	select {
	case <-apmServerTransportState.GracePeriodTimer.C:
		Log.Debug("Grace period over - timer timed out")
		return
	case <-ctx.Done():
		Log.Debug("Grace period over - context done")
		return
	}
}

// Warning : the apmServerTransportStatus state needs to be locked if this function is ever called
// concurrently in the future.
func enterBackoff(ctx context.Context) {
	apmServerTransportState.Lock()
	Log.Info("Entering backoff")
	if apmServerTransportState.Status == Healthy {
		apmServerTransportState.ReconnectionCount = 0
		Log.Debug("Entered backoff as healthy : reconnection count set to 0")
	} else {
		apmServerTransportState.ReconnectionCount++
		Log.Debugf("Entered backoff as pending : reconnection count set to %d", apmServerTransportState.ReconnectionCount)
	}
	apmServerTransportState.Status = Failing
	Log.Debug("Transport status set to failing")
	apmServerTransportState.GracePeriodTimer = time.NewTimer(computeGracePeriod())
	go func() {
		defer apmServerTransportState.Unlock()
		WaitForGracePeriod(ctx)
		apmServerTransportState.Status = Pending
		Log.Debug("Transport status set to pending")
	}()
}

// ComputeGracePeriod https://github.com/elastic/apm/blob/main/specs/agents/transport.md#transport-errors
func computeGracePeriod() time.Duration {
	gracePeriodWithoutJitter := math.Pow(math.Min(float64(apmServerTransportState.ReconnectionCount), 6), 2)
	jitter := rand.Float64()/5 - 0.1
	return time.Duration((gracePeriodWithoutJitter + jitter*gracePeriodWithoutJitter) * float64(time.Second))
}
