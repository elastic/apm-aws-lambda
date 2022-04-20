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

// MetricLabel is a name/value pair for labeling metrics.
type MetricLabel struct {
	// Name is the label name.
	Name string

	// Value is the label value.
	Value string
}

// MetricsTransaction holds transaction identifiers for metrics.
type MetricsTransaction struct {
	Type string `json:"type,omitempty"`
	Name string `json:"name,omitempty"`
}

// MetricsSpan holds span identifiers for metrics.
type MetricsSpan struct {
	Type    string `json:"type,omitempty"`
	Subtype string `json:"subtype,omitempty"`
}

// Metric holds metric values.
type Metric struct {
	Type string `json:"type,omitempty"`
	// Value holds the metric value.
	Value float64 `json:"value"`
	// Buckets holds the metric bucket values.
	Values []float64 `json:"values,omitempty"`
	// Count holds the metric observation count for the bucket.
	Counts []uint64 `json:"counts,omitempty"`
}

// Metrics holds a set of metric samples, with an optional set of labels.
type Metrics struct {
	// Timestamp holds the time at which the metric samples were taken.
	Timestamp int64 `json:"timestamp"`

	// Transaction optionally holds the name and type of transactions
	// with which these metrics are associated.
	Transaction MetricsTransaction `json:"transaction,omitempty"`

	// Span optionally holds the type and subtype of the spans with
	// which these metrics are associated.
	Span MetricsSpan `json:"span,omitempty"`

	// Labels holds a set of labels associated with the metrics.
	// The labels apply uniformly to all metric samples in the set.
	//
	// NOTE(axw) the schema calls the field "tags", but we use
	// "labels" for agent-internal consistency. Labels aligns better
	// with the common schema, anyway.
	Labels StringMap `json:"tags,omitempty"`

	// Samples holds a map of metric samples, keyed by metric name.
	Samples map[string]Metric `json:"samples"`
}
