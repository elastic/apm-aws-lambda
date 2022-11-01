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

package apmproxy_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elastic/apm-aws-lambda/accumulator"
	"github.com/elastic/apm-aws-lambda/apmproxy"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
)

func TestPostToApmServerDataCompressed(t *testing.T) {
	s := "A long time ago in a galaxy far, far away..."

	// Compress the data
	pr, pw := io.Pipe()
	gw, _ := gzip.NewWriterLevel(pw, gzip.BestSpeed)
	go func() {
		if _, err := gw.Write([]byte(s)); err != nil {
			t.Fail()
			return
		}
		if err := gw.Close(); err != nil {
			t.Fail()
			return
		}
		if err := pw.Close(); err != nil {
			t.Fail()
			return
		}
	}()

	// Create AgentData struct with compressed data
	data, _ := io.ReadAll(pr)
	agentData := accumulator.APMData{Data: data, ContentEncoding: "gzip"}

	// Create apm server and handler
	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bytes, _ := io.ReadAll(r.Body)
		assert.Equal(t, string(data), string(bytes))
		assert.Equal(t, "gzip", r.Header.Get("Content-Encoding"))
		w.WriteHeader(http.StatusAccepted)
		if _, err := w.Write([]byte(`{"foo": "bar"}`)); err != nil {
			t.Fail()
			return
		}
	}))
	defer apmServer.Close()

	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL(apmServer.URL),
		apmproxy.WithLogger(zaptest.NewLogger(t).Sugar()),
	)
	require.NoError(t, err)
	require.NoError(t, apmClient.PostToApmServer(context.Background(), agentData))
}

func TestPostToApmServerDataNotCompressed(t *testing.T) {
	s := "A long time ago in a galaxy far, far away..."
	body := []byte(s)
	agentData := accumulator.APMData{Data: body, ContentEncoding: ""}

	// Compress the data, so it can be compared with what
	// the apm server receives
	pr, pw := io.Pipe()
	gw, _ := gzip.NewWriterLevel(pw, gzip.BestSpeed)
	go func() {
		if _, err := gw.Write([]byte(s)); err != nil {
			t.Fail()
			return
		}
		if err := gw.Close(); err != nil {
			t.Fail()
			return
		}
		if err := pw.Close(); err != nil {
			t.Fail()
			return
		}
	}()

	// Create apm server and handler
	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestBytes, _ := io.ReadAll(r.Body)
		compressedBytes, _ := io.ReadAll(pr)
		assert.Equal(t, string(compressedBytes), string(requestBytes))
		assert.Equal(t, "gzip", r.Header.Get("Content-Encoding"))
		w.WriteHeader(http.StatusAccepted)
		if _, err := w.Write([]byte(`{"foo": "bar"}`)); err != nil {
			t.Fail()
			return
		}
	}))
	defer apmServer.Close()

	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL(apmServer.URL),
		apmproxy.WithLogger(zaptest.NewLogger(t).Sugar()),
	)
	require.NoError(t, err)
	require.NoError(t, apmClient.PostToApmServer(context.Background(), agentData))
}

func TestGracePeriod(t *testing.T) {
	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL("https://example.com"),
		apmproxy.WithLogger(zaptest.NewLogger(t).Sugar()),
	)
	require.NoError(t, err)

	apmClient.ReconnectionCount = 0
	val0 := apmClient.ComputeGracePeriod().Seconds()
	assert.LessOrEqual(t, val0, 5.0)

	apmClient.ReconnectionCount = 1
	val1 := apmClient.ComputeGracePeriod().Seconds()
	assert.InDelta(t, val1, float64(1), 0.1*1)

	apmClient.ReconnectionCount = 2
	val2 := apmClient.ComputeGracePeriod().Seconds()
	assert.InDelta(t, val2, float64(4), 0.1*4)

	apmClient.ReconnectionCount = 3
	val3 := apmClient.ComputeGracePeriod().Seconds()
	assert.InDelta(t, val3, float64(9), 0.1*9)

	apmClient.ReconnectionCount = 4
	val4 := apmClient.ComputeGracePeriod().Seconds()
	assert.InDelta(t, val4, float64(16), 0.1*16)

	apmClient.ReconnectionCount = 5
	val5 := apmClient.ComputeGracePeriod().Seconds()
	assert.InDelta(t, val5, float64(25), 0.1*25)

	apmClient.ReconnectionCount = 6
	val6 := apmClient.ComputeGracePeriod().Seconds()
	assert.InDelta(t, val6, float64(36), 0.1*36)

	apmClient.ReconnectionCount = 7
	val7 := apmClient.ComputeGracePeriod().Seconds()
	assert.InDelta(t, val7, float64(36), 0.1*36)
}

func TestSetHealthyTransport(t *testing.T) {
	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL("https://example.com"),
		apmproxy.WithLogger(zaptest.NewLogger(t).Sugar()),
	)
	require.NoError(t, err)
	apmClient.UpdateStatus(context.Background(), apmproxy.Healthy)
	assert.True(t, apmClient.Status == apmproxy.Healthy)
	assert.Equal(t, apmClient.ReconnectionCount, -1)
}

func TestSetFailingTransport(t *testing.T) {
	// By explicitly setting the reconnection count to 0, we ensure that the grace period will not be 0
	// and avoid a race between reaching the pending status and the test assertion.
	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL("https://example.com"),
		apmproxy.WithLogger(zaptest.NewLogger(t).Sugar()),
	)
	require.NoError(t, err)
	apmClient.ReconnectionCount = 0
	apmClient.UpdateStatus(context.Background(), apmproxy.Failing)
	assert.True(t, apmClient.Status == apmproxy.Failing)
	assert.Equal(t, apmClient.ReconnectionCount, 1)
}

func TestSetPendingTransport(t *testing.T) {
	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL("https://example.com"),
		apmproxy.WithLogger(zaptest.NewLogger(t).Sugar()),
	)
	require.NoError(t, err)
	apmClient.UpdateStatus(context.Background(), apmproxy.Healthy)
	apmClient.UpdateStatus(context.Background(), apmproxy.Failing)
	require.Eventually(t, func() bool {
		return !apmClient.IsUnhealthy()
	}, 7*time.Second, 50*time.Millisecond)
	assert.True(t, apmClient.Status == apmproxy.Started)
	assert.Equal(t, apmClient.ReconnectionCount, 0)
}

func TestSetPendingTransportExplicitly(t *testing.T) {
	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL("https://example.com"),
		apmproxy.WithLogger(zaptest.NewLogger(t).Sugar()),
	)
	require.NoError(t, err)
	apmClient.UpdateStatus(context.Background(), apmproxy.Healthy)
	apmClient.UpdateStatus(context.Background(), apmproxy.Started)
	assert.True(t, apmClient.Status == apmproxy.Healthy)
	assert.Equal(t, apmClient.ReconnectionCount, -1)
}

func TestSetInvalidTransport(t *testing.T) {
	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL("https://example.com"),
		apmproxy.WithLogger(zaptest.NewLogger(t).Sugar()),
	)
	require.NoError(t, err)
	apmClient.UpdateStatus(context.Background(), apmproxy.Healthy)
	apmClient.UpdateStatus(context.Background(), "Invalid")
	assert.True(t, apmClient.Status == apmproxy.Healthy)
	assert.Equal(t, apmClient.ReconnectionCount, -1)
}

func TestEnterBackoffFromHealthy(t *testing.T) {
	// Compress the data
	pr, pw := io.Pipe()
	gw, _ := gzip.NewWriterLevel(pw, gzip.BestSpeed)
	go func() {
		if _, err := gw.Write([]byte("")); err != nil {
			t.Fail()
			return
		}
		if err := gw.Close(); err != nil {
			t.Fail()
			return
		}
		if err := pw.Close(); err != nil {
			t.Fail()
			return
		}
	}()

	// Create AgentData struct with compressed data
	data, _ := io.ReadAll(pr)
	agentData := accumulator.APMData{Data: data, ContentEncoding: "gzip"}

	// Create apm server and handler
	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bytes, _ := io.ReadAll(r.Body)
		assert.Equal(t, string(data), string(bytes))
		assert.Equal(t, "gzip", r.Header.Get("Content-Encoding"))
		if _, err := w.Write([]byte(`{"foo": "bar"}`)); err != nil {
			return
		}
	}))

	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL(apmServer.URL),
		apmproxy.WithLogger(zaptest.NewLogger(t).Sugar()),
	)
	require.NoError(t, err)
	apmClient.UpdateStatus(context.Background(), apmproxy.Healthy)

	// Close the APM server early so that POST requests fail and that backoff is enabled
	apmServer.Close()

	if err := apmClient.PostToApmServer(context.Background(), agentData); err != nil {
		return
	}
	// No way to know for sure if failing or pending (0 sec grace period)
	assert.True(t, apmClient.Status != apmproxy.Healthy)
	assert.Equal(t, apmClient.ReconnectionCount, 0)
}

func TestEnterBackoffFromFailing(t *testing.T) {
	// Compress the data
	pr, pw := io.Pipe()
	gw, _ := gzip.NewWriterLevel(pw, gzip.BestSpeed)
	go func() {
		if _, err := gw.Write([]byte("")); err != nil {
			t.Fail()
			return
		}
		if err := gw.Close(); err != nil {
			t.Fail()
			return
		}
		if err := pw.Close(); err != nil {
			t.Fail()
			return
		}
	}()

	// Create AgentData struct with compressed data
	data, _ := io.ReadAll(pr)
	agentData := accumulator.APMData{Data: data, ContentEncoding: "gzip"}

	// Create apm server and handler
	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bytes, _ := io.ReadAll(r.Body)
		assert.Equal(t, string(data), string(bytes))
		assert.Equal(t, "gzip", r.Header.Get("Content-Encoding"))
		if _, err := w.Write([]byte(`{"foo": "bar"}`)); err != nil {
			t.Fail()
			return
		}
	}))
	// Close the APM server early so that POST requests fail and that backoff is enabled
	apmServer.Close()

	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL(apmServer.URL),
		apmproxy.WithLogger(zaptest.NewLogger(t).Sugar()),
	)
	require.NoError(t, err)

	apmClient.UpdateStatus(context.Background(), apmproxy.Healthy)
	apmClient.UpdateStatus(context.Background(), apmproxy.Failing)
	require.Eventually(t, func() bool {
		return !apmClient.IsUnhealthy()
	}, 7*time.Second, 50*time.Millisecond)
	assert.Equal(t, apmClient.Status, apmproxy.Started)

	assert.Error(t, apmClient.PostToApmServer(context.Background(), agentData))
	assert.Equal(t, apmClient.Status, apmproxy.Failing)
	assert.Equal(t, apmClient.ReconnectionCount, 1)
}

func TestAPMServerRecovery(t *testing.T) {
	// Compress the data
	pr, pw := io.Pipe()
	gw, _ := gzip.NewWriterLevel(pw, gzip.BestSpeed)
	go func() {
		if _, err := gw.Write([]byte("")); err != nil {
			t.Fail()
			return
		}
		if err := gw.Close(); err != nil {
			t.Fail()
			return
		}
		if err := pw.Close(); err != nil {
			t.Fail()
			return
		}
	}()

	// Create AgentData struct with compressed data
	data, _ := io.ReadAll(pr)
	agentData := accumulator.APMData{Data: data, ContentEncoding: "gzip"}

	// Create apm server and handler
	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bytes, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.Equal(t, string(data), string(bytes))
		assert.Equal(t, "gzip", r.Header.Get("Content-Encoding"))
		w.WriteHeader(http.StatusAccepted)
		if _, err := w.Write([]byte(`{"foo": "bar"}`)); err != nil {
			return
		}
	}))
	defer apmServer.Close()

	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL(apmServer.URL),
		apmproxy.WithLogger(zaptest.NewLogger(t).Sugar()),
	)
	require.NoError(t, err)

	apmClient.UpdateStatus(context.Background(), apmproxy.Healthy)
	apmClient.UpdateStatus(context.Background(), apmproxy.Failing)
	require.Eventually(t, func() bool {
		return !apmClient.IsUnhealthy()
	}, 7*time.Second, 50*time.Millisecond)
	assert.Equal(t, apmClient.Status, apmproxy.Started)

	assert.NoError(t, apmClient.PostToApmServer(context.Background(), agentData))
	assert.Equal(t, apmClient.Status, apmproxy.Healthy)
}

func TestAPMServerAuthFails(t *testing.T) {
	// Compress the data
	pr, pw := io.Pipe()
	gw, _ := gzip.NewWriterLevel(pw, gzip.BestSpeed)
	go func() {
		if _, err := gw.Write([]byte("")); err != nil {
			t.Fail()
			return
		}
		if err := gw.Close(); err != nil {
			t.Fail()
			return
		}
		if err := pw.Close(); err != nil {
			t.Fail()
			return
		}
	}()

	// Create AgentData struct with compressed data
	data, _ := io.ReadAll(pr)
	agentData := accumulator.APMData{Data: data, ContentEncoding: "gzip"}

	// Create apm server and handler
	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer apmServer.Close()

	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL(apmServer.URL),
		apmproxy.WithLogger(zaptest.NewLogger(t).Sugar()),
	)
	require.NoError(t, err)
	apmClient.UpdateStatus(context.Background(), apmproxy.Healthy)
	apmClient.UpdateStatus(context.Background(), apmproxy.Failing)
	require.Eventually(t, func() bool {
		return !apmClient.IsUnhealthy()
	}, 7*time.Second, 50*time.Millisecond)
	assert.Equal(t, apmClient.Status, apmproxy.Started)
	assert.NoError(t, apmClient.PostToApmServer(context.Background(), agentData))
	assert.NotEqual(t, apmClient.Status, apmproxy.Healthy)
}

func TestAPMServerRatelimit(t *testing.T) {
	// Compress the data
	pr, pw := io.Pipe()
	gw, _ := gzip.NewWriterLevel(pw, gzip.BestSpeed)
	go func() {
		if _, err := gw.Write([]byte("")); err != nil {
			t.Fail()
			return
		}
		if err := gw.Close(); err != nil {
			t.Fail()
			return
		}
		if err := pw.Close(); err != nil {
			t.Fail()
			return
		}
	}()

	// Create AgentData struct with compressed data
	data, _ := io.ReadAll(pr)
	agentData := accumulator.APMData{Data: data, ContentEncoding: "gzip"}

	// Create apm server and handler
	var shouldSucceed atomic.Bool
	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Fail the first request
		if shouldSucceed.CompareAndSwap(false, true) {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		w.WriteHeader(http.StatusAccepted)
	}))
	defer apmServer.Close()

	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL(apmServer.URL),
		apmproxy.WithLogger(zaptest.NewLogger(t).Sugar()),
	)
	require.NoError(t, err)
	assert.Equal(t, apmClient.Status, apmproxy.Started)

	// First request fails but does not trigger the backoff
	assert.NoError(t, apmClient.PostToApmServer(context.Background(), agentData))
	assert.Equal(t, apmClient.Status, apmproxy.RateLimited)

	// Followup request is succesful
	assert.NoError(t, apmClient.PostToApmServer(context.Background(), agentData))
	assert.Equal(t, apmClient.Status, apmproxy.Healthy)

}

func TestAPMServerClientFail(t *testing.T) {
	// Compress the data
	pr, pw := io.Pipe()
	gw, _ := gzip.NewWriterLevel(pw, gzip.BestSpeed)
	go func() {
		if _, err := gw.Write([]byte("")); err != nil {
			t.Fail()
			return
		}
		if err := gw.Close(); err != nil {
			t.Fail()
			return
		}
		if err := pw.Close(); err != nil {
			t.Fail()
			return
		}
	}()

	// Create AgentData struct with compressed data
	data, _ := io.ReadAll(pr)
	agentData := accumulator.APMData{Data: data, ContentEncoding: "gzip"}

	// Create apm server and handler
	var shouldSucceed atomic.Bool
	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Fail the first request
		if shouldSucceed.CompareAndSwap(false, true) {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}

		w.WriteHeader(http.StatusAccepted)
	}))
	defer apmServer.Close()

	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL(apmServer.URL),
		apmproxy.WithLogger(zaptest.NewLogger(t).Sugar()),
	)
	require.NoError(t, err)
	assert.Equal(t, apmClient.Status, apmproxy.Started)

	// First request fails but does not trigger the backoff
	assert.NoError(t, apmClient.PostToApmServer(context.Background(), agentData))
	assert.Equal(t, apmClient.Status, apmproxy.ClientFailing)

	// Followup request is succesful
	assert.NoError(t, apmClient.PostToApmServer(context.Background(), agentData))
	assert.Equal(t, apmClient.Status, apmproxy.Healthy)
}

func TestContinuedAPMServerFailure(t *testing.T) {
	// Compress the data
	pr, pw := io.Pipe()
	gw, _ := gzip.NewWriterLevel(pw, gzip.BestSpeed)
	go func() {
		if _, err := gw.Write([]byte("")); err != nil {
			t.Fail()
			return
		}
		if err := gw.Close(); err != nil {
			t.Fail()
			return
		}
		if err := pw.Close(); err != nil {
			t.Fail()
			return
		}
	}()

	// Create AgentData struct with compressed data
	data, _ := io.ReadAll(pr)
	agentData := accumulator.APMData{Data: data, ContentEncoding: "gzip"}

	// Create apm server and handler
	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bytes, _ := io.ReadAll(r.Body)
		assert.Equal(t, string(data), string(bytes))
		assert.Equal(t, "gzip", r.Header.Get("Content-Encoding"))
		if _, err := w.Write([]byte(`{"foo": "bar"}`)); err != nil {
			return
		}
	}))
	apmServer.Close()

	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL(apmServer.URL),
		apmproxy.WithLogger(zaptest.NewLogger(t).Sugar()),
	)
	require.NoError(t, err)
	apmClient.UpdateStatus(context.Background(), apmproxy.Healthy)
	apmClient.UpdateStatus(context.Background(), apmproxy.Failing)
	require.Eventually(t, func() bool {
		return !apmClient.IsUnhealthy()
	}, 7*time.Second, 50*time.Millisecond)
	assert.Equal(t, apmClient.Status, apmproxy.Started)
	assert.Error(t, apmClient.PostToApmServer(context.Background(), agentData))
	assert.Equal(t, apmClient.Status, apmproxy.Failing)
}

func TestForwardApmData(t *testing.T) {
	receivedReqBodyChan := make(chan []byte)
	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bytes, _ := io.ReadAll(r.Body)
		receivedReqBodyChan <- bytes
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(apmServer.Close)
	metadata := `{"metadata":{"service":{"agent":{"name":"apm-lambda-extension","version":"1.1.0"},"framework":{"name":"AWS Lambda","version":""},"language":{"name":"python","version":"3.9.8"},"runtime":{"name":"","version":""},"node":{}},"user":{},"process":{"pid":0},"system":{"container":{"id":""},"kubernetes":{"node":{},"pod":{}}},"cloud":{"provider":"","instance":{},"machine":{},"account":{},"project":{},"service":{}}}}`
	assertGzipBody := func(expected string) {
		var body []byte
		select {
		case body = <-receivedReqBodyChan:
		case <-time.After(1 * time.Second):
			require.Fail(t, "mock APM-Server timed out waiting for request")
		}
		buf := bytes.NewReader(body)
		r, err := gzip.NewReader(buf)
		require.NoError(t, err)
		out, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, expected, string(out))
	}
	agentData := fmt.Sprintf("%s\n%s", metadata, `{"transaction":{"id":"0102030405060708","trace_id":"0102030405060708090a0b0c0d0e0f10"}}`)
	lambdaData := `{"log": {"message": "test"}}`
	maxBatchAge := 1 * time.Second
	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL(apmServer.URL),
		apmproxy.WithLogger(zaptest.NewLogger(t).Sugar()),
		apmproxy.WithAgentDataBufferSize(10),
		// Configure a small batch age for ease of testing
		apmproxy.WithBatch(getReadyBatch(100, maxBatchAge)),
	)
	require.NoError(t, err)

	// Start forwarding APM data
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		assert.NoError(t, apmClient.ForwardApmData(ctx))
	}()

	// Populate metadata by sending agent data
	apmClient.AgentDataChannel <- accumulator.APMData{
		Data: []byte(agentData),
	}

	// Send lambda logs API data; the expected data will contain metadata
	// and agent data both.
	var expected bytes.Buffer
	expected.WriteString(agentData)
	// Send multiple lambda logs to batch data
	for i := 0; i < 5; i++ {
		if i == 4 {
			// Wait for batch age to make sure the batch is mature to be sent
			time.Sleep(maxBatchAge + time.Millisecond)
		}
		apmClient.LambdaDataChannel <- []byte(lambdaData)
		expected.WriteByte('\n')
		expected.WriteString(lambdaData)
	}

	assertGzipBody(expected.String())
	// Wait for ForwardApmData to exit
	cancel()
	wg.Wait()
}

func BenchmarkFlushAPMData(b *testing.B) {
	// Create apm server and handler
	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			return
		}
		if err := r.Body.Close(); err != nil {
			return
		}
		w.WriteHeader(202)
		if _, err := w.Write([]byte(`{}`)); err != nil {
			return
		}
	}))
	b.Cleanup(apmServer.Close)

	batch := getReadyBatch(100, time.Minute)
	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL(apmServer.URL),
		apmproxy.WithLogger(zaptest.NewLogger(b, zaptest.Level(zapcore.WarnLevel)).Sugar()),
		apmproxy.WithBatch(batch),
	)
	require.NoError(b, err)

	// Copied from https://github.com/elastic/apm-server/blob/master/testdata/intake-v2/transactions.ndjson.
	agentData := []byte(`{"metadata": {"service": {"name": "1234_service-12a3","node": {"configured_name": "node-123"},"version": "5.1.3","environment": "staging","language": {"name": "ecmascript","version": "8"},"runtime": {"name": "node","version": "8.0.0"},"framework": {"name": "Express","version": "1.2.3"},"agent": {"name": "elastic-node","version": "3.14.0"}},"user": {"id": "123user", "username": "bar", "email": "bar@user.com"}, "labels": {"tag0": null, "tag1": "one", "tag2": 2}, "process": {"pid": 1234,"ppid": 6789,"title": "node","argv": ["node","server.js"]},"system": {"hostname": "prod1.example.com","architecture": "x64","platform": "darwin", "container": {"id": "container-id"}, "kubernetes": {"namespace": "namespace1", "pod": {"uid": "pod-uid", "name": "pod-name"}, "node": {"name": "node-name"}}},"cloud":{"account":{"id":"account_id","name":"account_name"},"availability_zone":"cloud_availability_zone","instance":{"id":"instance_id","name":"instance_name"},"machine":{"type":"machine_type"},"project":{"id":"project_id","name":"project_name"},"provider":"cloud_provider","region":"cloud_region","service":{"name":"lambda"}}}}
{"transaction": { "id": "945254c567a5417e", "trace_id": "0123456789abcdef0123456789abcdef", "parent_id": "abcdefabcdef01234567", "type": "request", "duration": 32.592981,  "span_count": { "started": 43 }}}
{"transaction": {"id": "4340a8e0df1906ecbfa9", "trace_id": "0acd456789abcdef0123456789abcdef", "name": "GET /api/types","type": "request","duration": 32.592981,"outcome":"success", "result": "success", "timestamp": 1496170407154000, "sampled": true, "span_count": {"started": 17},"context": {"service": {"runtime": {"version": "7.0"}},"page":{"referer":"http://localhost:8000/test/e2e/","url":"http://localhost:8000/test/e2e/general-usecase/"}, "request": {"socket": {"remote_address": "12.53.12.1","encrypted": true},"http_version": "1.1","method": "POST","url": {"protocol": "https:","full": "https://www.example.com/p/a/t/h?query=string#hash","hostname": "www.example.com","port": "8080","pathname": "/p/a/t/h","search": "?query=string","hash": "#hash","raw": "/p/a/t/h?query=string#hash"},"headers": {"user-agent":["Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.103 Safari/537.36","Mozilla Chrome Edge"],"content-type": "text/html","cookie": "c1=v1, c2=v2","some-other-header": "foo","array": ["foo","bar","baz"]},"cookies": {"c1": "v1","c2": "v2"},"env": {"SERVER_SOFTWARE": "nginx","GATEWAY_INTERFACE": "CGI/1.1"},"body": {"str": "hello world","additional": { "foo": {},"bar": 123,"req": "additional information"}}},"response": {"status_code": 200,"headers": {"content-type": "application/json"},"headers_sent": true,"finished": true,"transfer_size":25.8,"encoded_body_size":26.90,"decoded_body_size":29.90}, "user": {"domain": "ldap://abc","id": "99","username": "foo"},"tags": {"organization_uuid": "9f0e9d64-c185-4d21-a6f4-4673ed561ec8", "tag2": 12, "tag3": 12.45, "tag4": false, "tag5": null },"custom": {"my_key": 1,"some_other_value": "foo bar","and_objects": {"foo": ["bar","baz"]},"(": "not a valid regex and that is fine"}}}}
{"transaction": { "id": "cdef4340a8e0df19", "trace_id": "0acd456789abcdef0123456789abcdef", "type": "request", "duration": 13.980558, "timestamp": 1532976822281000, "sampled": null, "span_count": { "dropped": 55, "started": 436 }, "marks": {"navigationTiming": {"appBeforeBootstrap": 608.9300000000001,"navigationStart": -21},"another_mark": {"some_long": 10,"some_float": 10.0}, "performance": {}}, "context": { "request": { "socket": { "remote_address": "192.0.1", "encrypted": null }, "method": "POST", "headers": { "user-agent": null, "content-type": null, "cookie": null }, "url": { "protocol": null, "full": null, "hostname": null, "port": null, "pathname": null, "search": null, "hash": null, "raw": null } }, "response": { "headers": { "content-type": null } }, "service": {"environment":"testing","name": "service1","node": {"configured_name": "node-ABC"}, "language": {"version": "2.5", "name": "ruby"}, "agent": {"version": "2.2", "name": "elastic-ruby", "ephemeral_id": "justanid"}, "framework": {"version": "5.0", "name": "Rails"}, "version": "2", "runtime": {"version": "2.5", "name": "cruby"}}},"experience":{"cls":1,"fid":2.0,"tbt":3.4,"longtask":{"count":3,"sum":2.5,"max":1}}}}
{"transaction": { "id": "00xxxxFFaaaa1234", "trace_id": "0123456789abcdef0123456789abcdef", "name": "amqp receive", "parent_id": "abcdefabcdef01234567", "type": "messaging", "duration": 3, "span_count": { "started": 1 }, "context": {"message": {"queue": { "name": "new_users"}, "age":{ "ms": 1577958057123}, "headers": {"user_id": "1ax3", "involved_services": ["user", "auth"]}, "body": "user created", "routing_key": "user-created-transaction"}},"session":{"id":"sunday","sequence":123}}}
{"transaction": { "name": "july-2021-delete-after-july-31", "type": "lambda", "result": "success", "id": "142e61450efb8574", "trace_id": "eb56529a1f461c5e7e2f66ecb075e983", "subtype": null, "action": null, "duration": 38.853, "timestamp": 1631736666365048, "sampled": true, "context": { "cloud": { "origin": { "account": { "id": "abc123" }, "provider": "aws", "region": "us-east-1", "service": { "name": "serviceName" } } }, "service": { "origin": { "id": "abc123", "name": "service-name", "version": "1.0" } }, "user": {}, "tags": {}, "custom": { } }, "sync": true, "span_count": { "started": 0 }, "outcome": "unknown", "faas": { "coldstart": false, "execution": "2e13b309-23e1-417f-8bf7-074fc96bc683", "trigger": { "request_id": "FuH2Cir_vHcEMUA=", "type": "http" } }, "sample_rate": 1 } }
`)
	agentAPMData := accumulator.APMData{Data: agentData}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		apmClient.AgentDataChannel <- agentAPMData
		for j := 0; j < 99; j++ {
			apmClient.LambdaDataChannel <- []byte(`{"log":{"message":this is test log"}}`)
		}
		apmClient.FlushAPMData(context.Background(), false)
	}
}

func BenchmarkPostToAPM(b *testing.B) {
	// Create apm server and handler
	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			return
		}
		if err := r.Body.Close(); err != nil {
			return
		}
		w.WriteHeader(202)
		if _, err := w.Write([]byte(`{}`)); err != nil {
			return
		}
	}))
	b.Cleanup(apmServer.Close)

	apmClient, err := apmproxy.NewClient(
		apmproxy.WithURL(apmServer.URL),
		apmproxy.WithLogger(zaptest.NewLogger(b, zaptest.Level(zapcore.WarnLevel)).Sugar()),
	)
	require.NoError(b, err)
	b.Cleanup(func() { apmClient.Shutdown() })

	// Copied from https://github.com/elastic/apm-server/blob/master/testdata/intake-v2/transactions.ndjson.
	benchBody := []byte(`{"metadata": {"service": {"name": "1234_service-12a3","node": {"configured_name": "node-123"},"version": "5.1.3","environment": "staging","language": {"name": "ecmascript","version": "8"},"runtime": {"name": "node","version": "8.0.0"},"framework": {"name": "Express","version": "1.2.3"},"agent": {"name": "elastic-node","version": "3.14.0"}},"user": {"id": "123user", "username": "bar", "email": "bar@user.com"}, "labels": {"tag0": null, "tag1": "one", "tag2": 2}, "process": {"pid": 1234,"ppid": 6789,"title": "node","argv": ["node","server.js"]},"system": {"hostname": "prod1.example.com","architecture": "x64","platform": "darwin", "container": {"id": "container-id"}, "kubernetes": {"namespace": "namespace1", "pod": {"uid": "pod-uid", "name": "pod-name"}, "node": {"name": "node-name"}}},"cloud":{"account":{"id":"account_id","name":"account_name"},"availability_zone":"cloud_availability_zone","instance":{"id":"instance_id","name":"instance_name"},"machine":{"type":"machine_type"},"project":{"id":"project_id","name":"project_name"},"provider":"cloud_provider","region":"cloud_region","service":{"name":"lambda"}}}}
{"transaction": { "id": "945254c567a5417e", "trace_id": "0123456789abcdef0123456789abcdef", "parent_id": "abcdefabcdef01234567", "type": "request", "duration": 32.592981,  "span_count": { "started": 43 }}}
{"transaction": {"id": "4340a8e0df1906ecbfa9", "trace_id": "0acd456789abcdef0123456789abcdef", "name": "GET /api/types","type": "request","duration": 32.592981,"outcome":"success", "result": "success", "timestamp": 1496170407154000, "sampled": true, "span_count": {"started": 17},"context": {"service": {"runtime": {"version": "7.0"}},"page":{"referer":"http://localhost:8000/test/e2e/","url":"http://localhost:8000/test/e2e/general-usecase/"}, "request": {"socket": {"remote_address": "12.53.12.1","encrypted": true},"http_version": "1.1","method": "POST","url": {"protocol": "https:","full": "https://www.example.com/p/a/t/h?query=string#hash","hostname": "www.example.com","port": "8080","pathname": "/p/a/t/h","search": "?query=string","hash": "#hash","raw": "/p/a/t/h?query=string#hash"},"headers": {"user-agent":["Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.103 Safari/537.36","Mozilla Chrome Edge"],"content-type": "text/html","cookie": "c1=v1, c2=v2","some-other-header": "foo","array": ["foo","bar","baz"]},"cookies": {"c1": "v1","c2": "v2"},"env": {"SERVER_SOFTWARE": "nginx","GATEWAY_INTERFACE": "CGI/1.1"},"body": {"str": "hello world","additional": { "foo": {},"bar": 123,"req": "additional information"}}},"response": {"status_code": 200,"headers": {"content-type": "application/json"},"headers_sent": true,"finished": true,"transfer_size":25.8,"encoded_body_size":26.90,"decoded_body_size":29.90}, "user": {"domain": "ldap://abc","id": "99","username": "foo"},"tags": {"organization_uuid": "9f0e9d64-c185-4d21-a6f4-4673ed561ec8", "tag2": 12, "tag3": 12.45, "tag4": false, "tag5": null },"custom": {"my_key": 1,"some_other_value": "foo bar","and_objects": {"foo": ["bar","baz"]},"(": "not a valid regex and that is fine"}}}}
{"transaction": { "id": "cdef4340a8e0df19", "trace_id": "0acd456789abcdef0123456789abcdef", "type": "request", "duration": 13.980558, "timestamp": 1532976822281000, "sampled": null, "span_count": { "dropped": 55, "started": 436 }, "marks": {"navigationTiming": {"appBeforeBootstrap": 608.9300000000001,"navigationStart": -21},"another_mark": {"some_long": 10,"some_float": 10.0}, "performance": {}}, "context": { "request": { "socket": { "remote_address": "192.0.1", "encrypted": null }, "method": "POST", "headers": { "user-agent": null, "content-type": null, "cookie": null }, "url": { "protocol": null, "full": null, "hostname": null, "port": null, "pathname": null, "search": null, "hash": null, "raw": null } }, "response": { "headers": { "content-type": null } }, "service": {"environment":"testing","name": "service1","node": {"configured_name": "node-ABC"}, "language": {"version": "2.5", "name": "ruby"}, "agent": {"version": "2.2", "name": "elastic-ruby", "ephemeral_id": "justanid"}, "framework": {"version": "5.0", "name": "Rails"}, "version": "2", "runtime": {"version": "2.5", "name": "cruby"}}},"experience":{"cls":1,"fid":2.0,"tbt":3.4,"longtask":{"count":3,"sum":2.5,"max":1}}}}
{"transaction": { "id": "00xxxxFFaaaa1234", "trace_id": "0123456789abcdef0123456789abcdef", "name": "amqp receive", "parent_id": "abcdefabcdef01234567", "type": "messaging", "duration": 3, "span_count": { "started": 1 }, "context": {"message": {"queue": { "name": "new_users"}, "age":{ "ms": 1577958057123}, "headers": {"user_id": "1ax3", "involved_services": ["user", "auth"]}, "body": "user created", "routing_key": "user-created-transaction"}},"session":{"id":"sunday","sequence":123}}}
{"transaction": { "name": "july-2021-delete-after-july-31", "type": "lambda", "result": "success", "id": "142e61450efb8574", "trace_id": "eb56529a1f461c5e7e2f66ecb075e983", "subtype": null, "action": null, "duration": 38.853, "timestamp": 1631736666365048, "sampled": true, "context": { "cloud": { "origin": { "account": { "id": "abc123" }, "provider": "aws", "region": "us-east-1", "service": { "name": "serviceName" } } }, "service": { "origin": { "id": "abc123", "name": "service-name", "version": "1.0" } }, "user": {}, "tags": {}, "custom": { } }, "sync": true, "span_count": { "started": 0 }, "outcome": "unknown", "faas": { "coldstart": false, "execution": "2e13b309-23e1-417f-8bf7-074fc96bc683", "trigger": { "request_id": "FuH2Cir_vHcEMUA=", "type": "http" } }, "sample_rate": 1 } }
`)
	agentData := accumulator.APMData{Data: benchBody, ContentEncoding: ""}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := apmClient.PostToApmServer(context.Background(), agentData); err != nil {
			b.Fatal(err)
		}
	}
}

func getReadyBatch(maxSize int, maxAge time.Duration) *accumulator.Batch {
	batch := accumulator.NewBatch(maxSize, maxAge)
	batch.RegisterInvocation("test-req-id", "test-func-arn", 10_000, time.Now())
	return batch
}
