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
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"elastic/apm-lambda-extension/extension"
	"elastic/apm-lambda-extension/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_processPlatformReport(t *testing.T) {

	timestamp := time.Now()

	pm := PlatformMetrics{
		DurationMs:       182.43,
		BilledDurationMs: 183,
		MemorySizeMB:     128,
		MaxMemoryUsedMB:  76,
		InitDurationMs:   422.97,
	}

	logEventRecord := LogEventRecord{
		RequestId: "6f7f0961f83442118a7af6fe80b88d56",
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

	desiredOutput := fmt.Sprintf(`{"metadata":{"service":{"agent":{"name":"apm-lambda-extension","version":"%s"},"framework":{"name":"AWS Lambda","version":""},"language":{"name":"python","version":"3.9.8"},"runtime":{"name":"","version":""},"node":{}},"user":{},"process":{"pid":0},"system":{"container":{"id":""},"kubernetes":{"node":{},"pod":{}}},"cloud":{"provider":"","instance":{},"machine":{},"account":{},"project":{},"service":{}}}}
{"metricset":{"timestamp":%d,"transaction":{},"span":{},"tags":{"faas.coldstart":true,"faas.execution":"6f7f0961f83442118a7af6fe80b88d56","faas.id":"arn:aws:lambda:us-east-2:123456789012:function:custom-runtime"},"samples":{"aws.lambda.metrics.BilledDuration":{"value":183},"aws.lambda.metrics.ColdStartDuration":{"value":422.9700012207031},"aws.lambda.metrics.Duration":{"value":182.42999267578125},"aws.lambda.metrics.Timeout":{"value":5000},"aws.lambda.metrics.TotalMemory":{"value":134217728},"aws.lambda.metrics.UsedMemory":{"value":79691776},"system.memory.actual.free":{"value":54525952},"system.memory.total":{"value":134217728}}}}`, extension.Version, timestamp.UnixNano()/1e3)

	mc := extension.MetadataContainer{
		Metadata: &model.Metadata{},
	}
	mc.Metadata.Service = model.Service{
		Name:      os.Getenv("AWS_LAMBDA_FUNCTION_NAME"),
		Agent:     model.Agent{Name: "python", Version: "6.7.2"},
		Language:  model.Language{Name: "python", Version: "3.9.8"},
		Runtime:   model.Runtime{Name: os.Getenv("AWS_EXECUTION_ENV")},
		Framework: model.Framework{Name: "AWS Lambda"},
	}

	rawBytes, err := ProcessPlatformReport(context.Background(), &mc, &event, logEvent)
	require.NoError(t, err)

	requestBytes, err := extension.GetUncompressedBytes(rawBytes.Data, "")
	require.NoError(t, err)

	assert.Equal(t, desiredOutput, string(requestBytes))
}
