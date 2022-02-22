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
