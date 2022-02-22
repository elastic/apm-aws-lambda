package extension

import (
	"elastic/apm-lambda-extension/logsapi"
	"elastic/apm-lambda-extension/model"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

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

func ProcessPlatformReport(client *http.Client, metadataContainer MetadataContainer, timestamp time.Time, platformReportMetrics logsapi.PlatformMetrics, config *extensionConfig) {

	metricsContainer := MetricsContainer{
		Metrics: &model.Metrics{},
	}
	convMB2Bytes := float64(1024 * 1024)

	// APM Spec Fields
	metricsContainer.Metrics.Timestamp = timestamp.UnixMicro()
	metricsContainer.Add("system.memory.total", nil, float64(platformReportMetrics.MemorySizeMB)*convMB2Bytes)                                             // Unit : Bytes
	metricsContainer.Add("system.memory.actual.free", nil, float64(platformReportMetrics.MemorySizeMB-platformReportMetrics.MaxMemoryUsedMB)*convMB2Bytes) // Unit : Bytes

	// Raw Metrics
	// AWS uses binary multiples to compute memory : https://aws.amazon.com/about-aws/whats-new/2020/12/aws-lambda-supports-10gb-memory-6-vcpu-cores-lambda-functions/
	metricsContainer.Add("faas.metrics.memory.total.bytes", nil, float64(platformReportMetrics.MemorySizeMB)*convMB2Bytes)      // Unit : Bytes
	metricsContainer.Add("faas.metrics.memory.maxUsed.bytes", nil, float64(platformReportMetrics.MaxMemoryUsedMB)*convMB2Bytes) // Unit : Bytes
	metricsContainer.Add("faas.metrics.duration.measured.ms", nil, float64(platformReportMetrics.DurationMs))                   // Unit : Milliseconds
	metricsContainer.Add("faas.metrics.duration.billed.ms", nil, float64(platformReportMetrics.BilledDurationMs))               // Unit : Milliseconds
	metricsContainer.Add("faas.metrics.duration.init.ms", nil, float64(platformReportMetrics.InitDurationMs))                   // Unit : Milliseconds

	metricsJson, err := json.Marshal(metricsContainer)
	if err != nil {
		log.Println(err)
		return
	}

	metadataJson, err := json.Marshal(metadataContainer)
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
