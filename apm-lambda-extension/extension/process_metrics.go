package extension

import (
	"elastic/apm-lambda-extension/logsapi"
	"elastic/apm-lambda-extension/model"
	"encoding/json"
	"log"
	"math"
	"net/http"
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

func ProcessPlatformReport(client *http.Client, metadataContainer MetadataContainer, functionData *NextEventResponse, platformReport logsapi.LogEvent, config *extensionConfig) {

	metricsContainer := MetricsContainer{
		Metrics: &model.Metrics{},
	}
	convMB2Bytes := float64(1024 * 1024)
	platformReportMetrics := platformReport.Record.Metrics

	// APM Spec Fields
	// Timestamp
	metricsContainer.Metrics.Timestamp = platformReport.Time.UnixNano() / 1e3
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
	metricsContainer.Add("aws.lambda.metrics.TotalMemory", nil, float64(platformReportMetrics.MemorySizeMB)*convMB2Bytes)                              // Unit : Bytes
	metricsContainer.Add("aws.lambda.metrics.UsedMemory", nil, float64(platformReportMetrics.MaxMemoryUsedMB)*convMB2Bytes)                            // Unit : Bytes
	metricsContainer.Add("aws.lambda.metrics.Duration", nil, float64(platformReportMetrics.DurationMs))                                                // Unit : Milliseconds
	metricsContainer.Add("aws.lambda.metrics.BilledDuration", nil, float64(platformReportMetrics.BilledDurationMs))                                    // Unit : Milliseconds
	metricsContainer.Add("aws.lambda.metrics.ColdStartDuration", nil, float64(platformReportMetrics.InitDurationMs))                                   // Unit : Milliseconds
	metricsContainer.Add("aws.lambda.metrics.Timeout", nil, math.Ceil(float64(functionData.DeadlineMs-functionData.Timestamp.UnixNano()/1e6)/1e3)*1e3) // Unit : Milliseconds

	metricsJson, err := json.Marshal(metricsContainer)
	if err != nil {
		log.Println(err)
		return
	}

	metadataContainer.Metadata.Service.Agent.Name = "aws-lambda-extension"
	metadataContainer.Metadata.Service.Agent.Version = Version

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
