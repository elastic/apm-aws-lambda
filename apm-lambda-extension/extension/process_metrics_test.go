package extension

import (
	"elastic/apm-lambda-extension/logsapi"
	"elastic/apm-lambda-extension/model"
	"fmt"
	"gotest.tools/assert"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func Test_processPlatformReport(t *testing.T) {

	pm := logsapi.PlatformMetrics{
		DurationMs:       182.43,
		BilledDurationMs: 183,
		MemorySizeMB:     128,
		MaxMemoryUsedMB:  76,
		InitDurationMs:   422.97,
	}

	timestamp := time.Now()

	desiredOutput := fmt.Sprintf(`{"metadata":{"service":{"agent":{"name":"python","version":"6.7.2"},"framework":{"name":"AWS Lambda","version":""},"language":{"name":"python","version":"3.9.8"},"runtime":{"name":"","version":""}},"process":{"pid":0},"system":{}}}
{"metricset":{"timestamp":%d,"transaction":{},"span":{},"samples":{"faas.metrics.duration.billed.ms":{"value":183},"faas.metrics.duration.init.ms":{"value":422.9700012207031},"faas.metrics.duration.measured.ms":{"value":182.42999267578125},"faas.metrics.memory.maxUsed.bytes":{"value":79691776},"faas.metrics.memory.total.bytes":{"value":134217728},"system.memory.actual.free":{"value":54525952},"system.memory.total":{"value":134217728}}}}`, timestamp.UnixMicro())

	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rawBytes []byte
		if r.Body != nil {
			rawBytes, _ = ioutil.ReadAll(r.Body)
		}
		requestBytes, err := getUncompressedBytes(rawBytes, r.Header.Get("Content-Encoding"))
		if err != nil {
			log.Println(err)
			t.Fail()
		}
		log.Printf("Test output : %s", string(requestBytes))
		assert.Equal(t, string(requestBytes), desiredOutput)
		assert.Equal(t, "gzip", r.Header.Get("Content-Encoding"))
		w.Write([]byte(`{"foo": "bar"}`))
	}))
	defer apmServer.Close()

	config := extensionConfig{
		apmServerUrl: apmServer.URL + "/",
	}

	mc := MetadataContainer{
		Metadata: &model.Metadata{},
	}
	mc.Metadata.Service = &model.Service{
		Name:      os.Getenv("AWS_LAMBDA_FUNCTION_NAME"),
		Agent:     &model.Agent{Name: "python", Version: "6.7.2"},
		Language:  &model.Language{Name: "python", Version: "3.9.8"},
		Runtime:   &model.Runtime{Name: os.Getenv("AWS_EXECUTION_ENV")},
		Framework: &model.Framework{Name: "AWS Lambda"},
	}
	mc.Metadata.Process = &model.Process{}
	mc.Metadata.System = &model.System{}

	ProcessPlatformReport(apmServer.Client(), mc, timestamp, pm, &config)
}
