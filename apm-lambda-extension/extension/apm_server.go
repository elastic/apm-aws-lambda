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

var ApmServerContext, _ = context.WithCancel(context.Background())

type ApmServerTransportStatusType string

const (
	TransportFailing ApmServerTransportStatusType = "TransportFailing"
	TransportPending ApmServerTransportStatusType = "TransportPending"
	TransportHealthy ApmServerTransportStatusType = "TransportHealthy"
)

var apmServerTransportStatus = TransportHealthy
var apmServerReconnectionCount = 0
var apmServerGracePeriodTimer *time.Timer

func SetApmServerTransportStatus(status ApmServerTransportStatusType, reconnectionCount int) {
	apmServerTransportStatus = status
	apmServerReconnectionCount = reconnectionCount
}

// todo: can this be a streaming or streaming style call that keeps the
//       connection open across invocations?
func PostToApmServer(client *http.Client, agentData AgentData, config *extensionConfig, ctx context.Context) error {
	if !IsTransportStatusHealthy() {
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

	apmServerTransportStatus = TransportHealthy
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
		Log.Warn("Channel full: dropping a subset of agent request data")
	}
}

func IsTransportStatusHealthy() bool {
	return apmServerTransportStatus != TransportFailing
}

func WaitForGracePeriod(ctx context.Context) {
	select {
	case <-apmServerGracePeriodTimer.C:
		Log.Debug("Grace period over - timer timed out")
		return
	case <-ctx.Done():
		Log.Debug("Grace period over - context done")
		return
	}

}

func enterBackoff(ctx context.Context) {
	Log.Info("Entering backoff")
	if apmServerTransportStatus == TransportHealthy {
		apmServerReconnectionCount = 0
	} else {
		apmServerReconnectionCount++
	}
	apmServerTransportStatus = TransportFailing
	Log.Debug("Transport status set to failing")
	apmServerGracePeriodTimer = time.NewTimer(computeGracePeriod())
	go func() {
		WaitForGracePeriod(ctx)
		apmServerTransportStatus = TransportPending
		Log.Debug("Transport status set to pending")
	}()
}

// ComputeGracePeriod https://github.com/elastic/apm/blob/main/specs/agents/transport.md#transport-errors
func computeGracePeriod() time.Duration {
	gracePeriodWithoutJitter := math.Pow(math.Min(float64(apmServerReconnectionCount), 6), 2)
	jitter := rand.Float64()/5 - 0.1
	return time.Duration((gracePeriodWithoutJitter + jitter*gracePeriodWithoutJitter) * float64(time.Second))
}
