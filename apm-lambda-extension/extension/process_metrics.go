package extension

import (
	"elastic/apm-lambda-extension/logsapi"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

// Code reused from the Go Agent codebase : https://github.com/elastic/apm-agent-go/blob/main/model/model.go
type MetadataContainer struct {
	Metadata *Metadata `json:"metadata"`
}

type Metadata struct {
	Service *Service `json:"service"`
	Process *Process `json:"process"`
	System  *System  `json:"system"`
}

// Service represents the service handling transactions being traced.
type Service struct {
	// Name is the immutable name of the service.
	Name string `json:"name,omitempty"`

	// Version is the version of the service, if it has one.
	Version string `json:"version,omitempty"`

	// Environment is the name of the service's environment, if it has
	// one, e.g. "production" or "staging".
	Environment string `json:"environment,omitempty"`

	// Agent holds information about the Elastic APM agent tracing this
	// service's transactions.
	Agent *Agent `json:"agent,omitempty"`

	// Framework holds information about the service's framework, if any.
	Framework *Framework `json:"framework,omitempty"`

	// Language holds information about the programming language in which
	// the service is written.
	Language *Language `json:"language,omitempty"`

	// Runtime holds information about the programming language runtime
	// running this service.
	Runtime *Runtime `json:"runtime,omitempty"`
}

// Agent holds information about the Elastic APM agent.
type Agent struct {
	// Name is the name of the Elastic APM agent, e.g. "Go".
	Name string `json:"name"`

	// Version is the version of the Elastic APM agent, e.g. "1.0.0".
	Version string `json:"version"`
}

// Framework holds information about the framework (typically web)
// used by the service.
type Framework struct {
	// Name is the name of the framework.
	Name string `json:"name"`

	// Version is the version of the framework.
	Version string `json:"version"`
}

// Language holds information about the programming language used.
type Language struct {
	// Name is the name of the programming language.
	Name string `json:"name"`

	// Version is the version of the programming language.
	Version string `json:"version,omitempty"`
}

// Runtime holds information about the programming language runtime.
type Runtime struct {
	// Name is the name of the programming language runtime.
	Name string `json:"name"`

	// Version is the version of the programming language runtime.
	Version string `json:"version"`
}

// System represents the system (operating system and machine) running the
// service.
type System struct {
	// Architecture is the system's hardware architecture.
	Architecture string `json:"architecture,omitempty"`

	// Hostname is the system's hostname.
	Hostname string `json:"hostname,omitempty"`

	// Platform is the system's platform, or operating system name.
	Platform string `json:"platform,omitempty"`
}

// Process represents an operating system process.
type Process struct {
	// Pid is the process ID.
	Pid int `json:"pid,omitempty"`

	// Ppid is the parent process ID, if known.
	Ppid *int `json:"ppid,omitempty"`

	// Title is the title of the process.
	Title string `json:"title,omitempty"`

	// Argv holds the command line arguments used to start the process.
	Argv []string `json:"argv,omitempty"`
}

// StringMap is a slice-representation of map[string]string,
// optimized for fast JSON encoding.
//
// Slice items are expected to be ordered by key.
type StringMap []StringMapItem

// StringMapItem holds a string key and value.
type StringMapItem struct {
	// Key is the map item's key.
	Key string

	// Value is the map item's value.
	Value string
}

// MetricLabel is a name/value pair for labeling metrics.
type MetricLabel struct {
	// Name is the label name.
	Name string

	// Value is the label value.
	Value string
}

type MetricsContainer struct {
	Metrics *Metrics `json:"metricset"`
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

// Add adds a metric with the given name, labels, and value,
// The labels are expected to be sorted lexicographically.
func (m *Metrics) Add(name string, labels []MetricLabel, value float64) {
	m.addMetric(name, labels, Metric{Value: value})
}

// AddHistogram adds a histogram metric with the given name, labels, counts,
// and values. The labels are expected to be sorted lexicographically, and
// bucket values provided in ascending order.
func (m *Metrics) AddHistogram(name string, labels []MetricLabel, values []float64, counts []uint64) {
	m.addMetric(name, labels, Metric{Values: values, Counts: counts, Type: "histogram"})
}

// Simplified version of https://github.com/elastic/apm-agent-go/blob/675e8398c7fe546f9fd169bef971b9ccfbcdc71f/metrics.go#L89
func (m *Metrics) addMetric(name string, labels []MetricLabel, metric Metric) {

	var modelLabels StringMap
	if len(labels) > 0 {
		modelLabels = make(StringMap, len(labels))
		for i, l := range labels {
			modelLabels[i] = StringMapItem{
				Key: l.Name, Value: l.Value,
			}
		}
	}

	m.Labels = modelLabels
	if m.Samples == nil {
		m.Samples = make(map[string]Metric)
	}
	m.Samples[name] = metric
}

func ProcessPlatformReport(client *http.Client, timestamp time.Time, platformReportMetrics logsapi.PlatformMetrics, config *extensionConfig) {

	var metadata Metadata
	metadata.Service = &Service{
		Name:      os.Getenv("AWS_LAMBDA_FUNCTION_NAME"),
		Agent:     &Agent{Name: "python", Version: "6.7.2"},
		Language:  &Language{Name: "python", Version: "3.9.8"},
		Runtime:   &Runtime{Name: os.Getenv("AWS_EXECUTION_ENV")},
		Framework: &Framework{Name: "AWS Lambda"},
	}
	metadata.Process = &Process{}
	metadata.System = &System{}

	metadataJson, err := json.Marshal(MetadataContainer{Metadata: &metadata})
	if err != nil {
		log.Println(err)
		return
	}

	var metrics Metrics
	convMB2Bytes := float64(1024 * 1024)

	// APM Spec Fields
	metrics.Timestamp = timestamp.UnixMicro()
	metrics.Add("system.memory.total", nil, float64(platformReportMetrics.MemorySizeMB)*convMB2Bytes)                                             // Unit : Bytes
	metrics.Add("system.memory.actual.free", nil, float64(platformReportMetrics.MemorySizeMB-platformReportMetrics.MaxMemoryUsedMB)*convMB2Bytes) // Unit : Bytes

	// Raw Metrics
	// AWS uses binary multiples to compute memory : https://aws.amazon.com/about-aws/whats-new/2020/12/aws-lambda-supports-10gb-memory-6-vcpu-cores-lambda-functions/
	metrics.Add("faas.metrics.memory.total.bytes", nil, float64(platformReportMetrics.MemorySizeMB)*convMB2Bytes)      // Unit : Bytes
	metrics.Add("faas.metrics.memory.maxUsed.bytes", nil, float64(platformReportMetrics.MaxMemoryUsedMB)*convMB2Bytes) // Unit : Bytes
	metrics.Add("faas.metrics.duration.measured.ms", nil, float64(platformReportMetrics.DurationMs))                   // Unit : Milliseconds
	metrics.Add("faas.metrics.duration.billed.ms", nil, float64(platformReportMetrics.BilledDurationMs))               // Unit : Milliseconds
	metrics.Add("faas.metrics.duration.init.ms", nil, float64(platformReportMetrics.InitDurationMs))                   // Unit : Milliseconds

	metricsJson, err := json.Marshal(MetricsContainer{Metrics: &metrics})
	if err != nil {
		log.Println(err)
		return
	}

	metricsData := append(metadataJson, []byte("\n")...)
	metricsData = append(metricsData, metricsJson...)

	err = PostToApmServer(client, AgentData{Data: metricsData}, config)
	if err != nil {
		log.Println(err)
		return
	}
}
