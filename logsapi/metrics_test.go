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

import (
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/elastic/apm-aws-lambda/apmproxy"
	"github.com/elastic/apm-aws-lambda/extension"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessPlatformReport_Coldstart(t *testing.T) {
	timestamp := time.Now()

	pm := PlatformMetrics{
		DurationMs:       182.43,
		BilledDurationMs: 183,
		MemorySizeMB:     128,
		MaxMemoryUsedMB:  76,
		InitDurationMs:   422.97,
	}

	logEventRecord := LogEventRecord{
		RequestID: "6f7f0961f83442118a7af6fe80b88d56",
		Status:    "Available",
		Metrics:   pm,
	}

	logEvent := LogEvent{
		Time:         timestamp,
		Type:         "platform.report",
		StringRecord: "",
		Record:       logEventRecord,
	}

	event := extension.NextEventResponse{
		Timestamp:          timestamp,
		EventType:          extension.Invoke,
		DeadlineMs:         timestamp.UnixNano()/1e6 + 4584, // Milliseconds
		RequestID:          "8476a536-e9f4-11e8-9739-2dfe598c3fcd",
		InvokedFunctionArn: "arn:aws:lambda:us-east-2:123456789012:function:custom-runtime",
		Tracing: extension.Tracing{
			Type:  "None",
			Value: "None",
		},
	}

	desiredOutputMetrics := fmt.Sprintf(`{"metricset":{"samples":{"faas.coldstart_duration":{"value":422.9700012207031},"faas.timeout":{"value":5000},"system.memory.total":{"value":1.34217728e+08},"system.memory.actual.free":{"value":5.4525952e+07},"faas.duration":{"value":182.42999267578125},"faas.billed_duration":{"value":183}},"timestamp":%d,"faas":{"coldstart":true,"execution":"6f7f0961f83442118a7af6fe80b88d56","id":"arn:aws:lambda:us-east-2:123456789012:function:custom-runtime"}}}`, timestamp.UnixNano()/1e3)

	apmData, err := ProcessPlatformReport(&event, logEvent)
	require.NoError(t, err)
	assert.Equal(t, apmproxy.Lambda, apmData.Type)

	requestBytes, err := apmproxy.GetUncompressedBytes(apmData.Data, "")
	require.NoError(t, err)

	out := string(requestBytes)
	log.Println(out)

	assert.JSONEq(t, desiredOutputMetrics, string(requestBytes))
}

func TestProcessPlatformReport_NoColdstart(t *testing.T) {
	timestamp := time.Now()

	pm := PlatformMetrics{
		DurationMs:       182.43,
		BilledDurationMs: 183,
		MemorySizeMB:     128,
		MaxMemoryUsedMB:  76,
		InitDurationMs:   0,
	}

	logEventRecord := LogEventRecord{
		RequestID: "6f7f0961f83442118a7af6fe80b88d56",
		Status:    "Available",
		Metrics:   pm,
	}

	logEvent := LogEvent{
		Time:         timestamp,
		Type:         "platform.report",
		StringRecord: "",
		Record:       logEventRecord,
	}

	event := extension.NextEventResponse{
		Timestamp:          timestamp,
		EventType:          extension.Invoke,
		DeadlineMs:         timestamp.UnixNano()/1e6 + 4584, // Milliseconds
		RequestID:          "8476a536-e9f4-11e8-9739-2dfe598c3fcd",
		InvokedFunctionArn: "arn:aws:lambda:us-east-2:123456789012:function:custom-runtime",
		Tracing: extension.Tracing{
			Type:  "None",
			Value: "None",
		},
	}

	desiredOutputMetrics := fmt.Sprintf(`{"metricset":{"samples":{"faas.coldstart_duration":{"value":0},"faas.timeout":{"value":5000},"system.memory.total":{"value":1.34217728e+08},"system.memory.actual.free":{"value":5.4525952e+07},"faas.duration":{"value":182.42999267578125},"faas.billed_duration":{"value":183}},"timestamp":%d,"faas":{"coldstart":false,"execution":"6f7f0961f83442118a7af6fe80b88d56","id":"arn:aws:lambda:us-east-2:123456789012:function:custom-runtime"}}}`, timestamp.UnixNano()/1e3)

	apmData, err := ProcessPlatformReport(&event, logEvent)
	require.NoError(t, err)
	assert.Equal(t, apmproxy.Lambda, apmData.Type)

	requestBytes, err := apmproxy.GetUncompressedBytes(apmData.Data, "")
	require.NoError(t, err)

	assert.JSONEq(t, desiredOutputMetrics, string(requestBytes))
}

func BenchmarkPlatformReport(b *testing.B) {
	reqID := "8476a536-e9f4-11e8-9739-2dfe598c3fcd"
	invokedFnArn := "arn:aws:lambda:us-east-2:123456789012:function:custom-runtime"
	timestamp := time.Date(2022, 11, 12, 0, 0, 0, 0, time.UTC)
	logEvent := LogEvent{
		Time: timestamp,
		Type: PlatformReport,
		Record: LogEventRecord{
			RequestID: reqID,
			Metrics: PlatformMetrics{
				DurationMs:       1.0,
				BilledDurationMs: 1,
				MemorySizeMB:     1,
				MaxMemoryUsedMB:  1,
				InitDurationMs:   1.0,
			},
		},
	}
	nextEventResp := &extension.NextEventResponse{
		Timestamp:          timestamp,
		EventType:          extension.Invoke,
		RequestID:          reqID,
		InvokedFunctionArn: invokedFnArn,
	}

	for n := 0; n < b.N; n++ {
		_, err := ProcessPlatformReport(nextEventResp, logEvent)
		require.NoError(b, err)
	}
}
