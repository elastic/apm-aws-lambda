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

	"github.com/elastic/apm-aws-lambda/apmproxy"
	"github.com/elastic/apm-aws-lambda/extension"
	"go.elastic.co/apm/v2/model"
	"go.elastic.co/fastjson"
)

type PlatformMetrics struct {
	DurationMs       float32 `json:"durationMs"`
	BilledDurationMs int32   `json:"billedDurationMs"`
	MemorySizeMB     int32   `json:"memorySizeMB"`
	MaxMemoryUsedMB  int32   `json:"maxMemoryUsedMB"`
	InitDurationMs   float32 `json:"initDurationMs"`
}

type MetricsContainer struct {
	Metrics *model.Metrics `json:"metricset"`
}

// Add adds a metric with the given name, labels, and value,
// The labels are expected to be sorted lexicographically.
func (mc MetricsContainer) Add(name string, value float64) {
	mc.addMetric(name, model.Metric{Value: value})
}

// Simplified version of https://github.com/elastic/apm-agent-go/blob/675e8398c7fe546f9fd169bef971b9ccfbcdc71f/metrics.go#L89
func (mc MetricsContainer) addMetric(name string, metric model.Metric) {

	if mc.Metrics.Samples == nil {
		mc.Metrics.Samples = make(map[string]model.Metric)
	}
	mc.Metrics.Samples[name] = metric
}

func (mc MetricsContainer) MarshalFastJSON(json *fastjson.Writer) error {
	json.RawString(`{"metricset":`)
	if err := mc.Metrics.MarshalFastJSON(json); err != nil {
		return err
	}
	json.RawString(`}`)
	return nil
}

func ProcessPlatformReport(metadataContainer *apmproxy.MetadataContainer, functionData *extension.NextEventResponse, platformReport LogEvent) (apmproxy.AgentData, error) {
	metricsContainer := MetricsContainer{
		Metrics: &model.Metrics{},
	}
	convMB2Bytes := float64(1024 * 1024)
	platformReportMetrics := platformReport.Record.Metrics

	// APM Spec Fields
	// Timestamp
	metricsContainer.Metrics.Timestamp = model.Time(platformReport.Time)

	// FaaS Fields
	metricsContainer.Metrics.FAAS = &model.FAAS{
		Execution: platformReport.Record.RequestID,
		ID:        functionData.InvokedFunctionArn,
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
	metricsContainer.Add("faas.timeout", math.Ceil(float64(functionData.DeadlineMs-functionData.Timestamp.UnixMilli())/1e3)*1e3) // Unit : Milliseconds

	var jsonWriter fastjson.Writer
	if err := metricsContainer.MarshalFastJSON(&jsonWriter); err != nil {
		return apmproxy.AgentData{}, err
	}

	capacity := len(metadataContainer.Metadata) + jsonWriter.Size() + 1 // 1 for newline
	metricsData := make([]byte, len(metadataContainer.Metadata), capacity)
	copy(metricsData, metadataContainer.Metadata)

	metricsData = append(metricsData, []byte("\n")...)
	metricsData = append(metricsData, jsonWriter.Bytes()...)
	return apmproxy.AgentData{Data: metricsData}, nil
}
