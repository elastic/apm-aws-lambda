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

package model

import (
	"time"

	"go.elastic.co/fastjson"
)

type LogContainer struct {
	Log *LogLine `json:"log,omitempty"`
}

type LogLine struct {
	Message   string `json:"message"`
	Timestamp Time   `json:"@timestamp"`
	FAAS      *FAAS  `json:"faas,omitempty"`
}

type Time time.Time

func (t Time) MarshalFastJSON(w *fastjson.Writer) error {
	w.Int64(time.Time(t).UnixNano() / int64(time.Microsecond))
	return nil
}

// faas struct is a subset of go.elastic.co/apm/v2/model#FAAS
//
// The purpose of having a separate struct is to have a custom
// marshalling logic that is targeted for the faas fields
// available for function logs. For example: `coldstart` value
// cannot be inferred for function logs so this struct drops
// the field entirely.
type FAAS struct {
	// ID holds a unique identifier of the invoked serverless function.
	ID string `json:"id,omitempty"`
	// Execution holds the request ID of the function invocation.
	Execution string `json:"execution,omitempty"`
}

type MetricsContainer struct {
	Metrics *Metrics `json:"metricset,omitempty"`
}

// Add adds a metric with the given name, labels, and value,
// The labels are expected to be sorted lexicographically.
func (mc MetricsContainer) Add(name string, value float64) {
	mc.addMetric(name, Metric{Value: value})
}

// Simplified version of https://github.com/elastic/apm-agent-go/blob/675e8398c7fe546f9fd169bef971b9ccfbcdc71f/metrics.go#L89
func (mc MetricsContainer) addMetric(name string, metric Metric) {
	if mc.Metrics.Samples == nil {
		mc.Metrics.Samples = make(map[string]Metric)
	}
	mc.Metrics.Samples[name] = metric
}

type Metrics struct {
	Timestamp Time              `json:"timestamp"`
	FAAS      *ExtendedFAAS     `json:"faas,omitempty"`
	Samples   map[string]Metric `json:"samples,omitempty"`
}

type ExtendedFAAS struct {
	ID        string `json:"id,omitempty"`
	Execution string `json:"execution,omitempty"`
	Coldstart bool   `json:"coldstart"`
}

type Metric struct {
	Value float64 `json:"value"`
}
