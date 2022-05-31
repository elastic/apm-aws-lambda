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
	"encoding/json"
	"math"

	"elastic/apm-lambda-extension/extension"
	"elastic/apm-lambda-extension/model"
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
func (mc MetricsContainer) Add(name string, labels []model.MetricLabel, value float64) {
	mc.addMetric(name, model.Metric{Value: value})
}

// Simplified version of https://github.com/elastic/apm-agent-go/blob/675e8398c7fe546f9fd169bef971b9ccfbcdc71f/metrics.go#L89
func (mc MetricsContainer) addMetric(name string, metric model.Metric) {

	if mc.Metrics.Samples == nil {
		mc.Metrics.Samples = make(map[string]model.Metric)
	}
	mc.Metrics.Samples[name] = metric
}

func ProcessPlatformReport(ctx context.Context, apmServerTransport *extension.ApmServerTransport, metadataContainer *extension.MetadataContainer, functionData *extension.NextEventResponse, platformReport LogEvent) error {
	var metricsData []byte
	metricsContainer := MetricsContainer{
		Metrics: &model.Metrics{},
	}
	convMB2Bytes := float64(1024 * 1024)
	platformReportMetrics := platformReport.Record.Metrics

	// APM Spec Fields
	// Timestamp
	metricsContainer.Metrics.Timestamp = platformReport.Time.UnixMicro()
	// FaaS Fields
	metricsContainer.Metrics.Labels = make(map[string]interface{})
	metricsContainer.Metrics.Labels["faas.execution"] = platformReport.Record.RequestId
	metricsContainer.Metrics.Labels["faas.id"] = functionData.InvokedFunctionArn
	if platformReportMetrics.InitDurationMs > 0 {
		metricsContainer.Metrics.Labels["faas.coldstart"] = true
	} else {
		metricsContainer.Metrics.Labels["faas.coldstart"] = false
	}
	// System
	metricsContainer.Add("system.memory.total", nil, float64(platformReportMetrics.MemorySizeMB)*convMB2Bytes)                                             // Unit : Bytes
	metricsContainer.Add("system.memory.actual.free", nil, float64(platformReportMetrics.MemorySizeMB-platformReportMetrics.MaxMemoryUsedMB)*convMB2Bytes) // Unit : Bytes

	// Raw Metrics
	// AWS uses binary multiples to compute memory : https://aws.amazon.com/about-aws/whats-new/2020/12/aws-lambda-supports-10gb-memory-6-vcpu-cores-lambda-functions/
	metricsContainer.Add("aws.lambda.metrics.TotalMemory", nil, float64(platformReportMetrics.MemorySizeMB)*convMB2Bytes)                           // Unit : Bytes
	metricsContainer.Add("aws.lambda.metrics.UsedMemory", nil, float64(platformReportMetrics.MaxMemoryUsedMB)*convMB2Bytes)                         // Unit : Bytes
	metricsContainer.Add("aws.lambda.metrics.Duration", nil, float64(platformReportMetrics.DurationMs))                                             // Unit : Milliseconds
	metricsContainer.Add("aws.lambda.metrics.BilledDuration", nil, float64(platformReportMetrics.BilledDurationMs))                                 // Unit : Milliseconds
	metricsContainer.Add("aws.lambda.metrics.ColdStartDuration", nil, float64(platformReportMetrics.InitDurationMs))                                // Unit : Milliseconds
	metricsContainer.Add("aws.lambda.metrics.Timeout", nil, math.Ceil(float64(functionData.DeadlineMs-functionData.Timestamp.UnixMilli())/1e3)*1e3) // Unit : Milliseconds

	metricsJson, err := json.Marshal(metricsContainer)
	if err != nil {
		return err
	}

	if metadataContainer.Metadata != nil {
		//TODO : Discuss relevance of displaying extension name
		metadataContainer.Metadata.Service.Agent.Name = "aws-lambda-extension"
		metadataContainer.Metadata.Service.Agent.Version = extension.Version
		metadataJson, err := json.Marshal(metadataContainer)
		if err != nil {
			return err
		}
		metricsData = append(metadataJson, []byte("\n")...)
	}

	metricsData = append(metricsData, metricsJson...)
	apmServerTransport.EnqueueAPMData(extension.AgentData{Data: metricsData})
	return nil
}
