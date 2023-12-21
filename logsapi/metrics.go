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
	"math"
	"time"

	"github.com/elastic/apm-aws-lambda/logsapi/model"
	"go.elastic.co/fastjson"
)

type PlatformMetrics struct {
	DurationMs       float32 `json:"durationMs"`
	BilledDurationMs int32   `json:"billedDurationMs"`
	MemorySizeMB     int32   `json:"memorySizeMB"`
	MaxMemoryUsedMB  int32   `json:"maxMemoryUsedMB"`
	InitDurationMs   float32 `json:"initDurationMs"`
}

// ProcessPlatformReport processes the `platform.report` log line from lambda logs API and
// returns a byte array containing the JSON body for the extracted platform metrics. A non
// nil error is returned when marshaling of platform metrics into JSON fails.
func ProcessPlatformReport(fnARN string, deadlineMs int64, ts time.Time, platformReport LogEvent) ([]byte, error) {
	metricsContainer := model.MetricsContainer{
		Metrics: &model.Metrics{},
	}
	convMB2Bytes := float64(1024 * 1024)
	platformReportMetrics := platformReport.Record.Metrics

	// APM Spec Fields
	// Timestamp
	metricsContainer.Metrics.Timestamp = model.Time(platformReport.Time)

	// FaaS Fields
	metricsContainer.Metrics.FAAS = &model.ExtendedFAAS{
		Execution: platformReport.Record.RequestID,
		ID:        fnARN,
		Coldstart: platformReportMetrics.InitDurationMs > 0,
	}

	// System
	// AWS uses binary multiples to compute memory : https://aws.amazon.com/about-aws/whats-new/2020/12/aws-lambda-supports-10gb-memory-6-vcpu-cores-lambda-functions/
	metricsContainer.Add("system.memory.total", float64(platformReportMetrics.MemorySizeMB)*convMB2Bytes)                                             // Unit : Bytes
	metricsContainer.Add("system.memory.actual.free", float64(platformReportMetrics.MemorySizeMB-platformReportMetrics.MaxMemoryUsedMB)*convMB2Bytes) // Unit : Bytes

	// Raw Metrics
	metricsContainer.Add("faas.duration", float64(platformReportMetrics.DurationMs))               // Unit : Milliseconds
	metricsContainer.Add("faas.billed_duration", float64(platformReportMetrics.BilledDurationMs))  // Unit : Milliseconds
	metricsContainer.Add("faas.coldstart_duration", float64(platformReportMetrics.InitDurationMs)) // Unit : Milliseconds
	// In AWS Lambda, the Timeout is configured as an integer number of seconds. We use this assumption to derive the Timeout from
	// - The epoch corresponding to the end of the current invocation (its "deadline")
	// - The epoch corresponding to the start of the current invocation
	// - The multiplication / division then rounds the value to obtain a number of ms that can be expressed a multiple of 1000 (see initial assumption)
	metricsContainer.Add("faas.timeout", math.Ceil(float64(deadlineMs-ts.UnixMilli())/1e3)*1e3) // Unit : Milliseconds

	var jsonWriter fastjson.Writer
	if err := metricsContainer.MarshalFastJSON(&jsonWriter); err != nil {
		return nil, err
	}

	return jsonWriter.Bytes(), nil
}
