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

	timestamp := time.Now()

	pm := logsapi.PlatformMetrics{
		DurationMs:       182.43,
		BilledDurationMs: 183,
		MemorySizeMB:     128,
		MaxMemoryUsedMB:  76,
		InitDurationMs:   422.97,
	}

	logEventRecord := logsapi.LogEventRecord{
		RequestId: "6f7f0961f83442118a7af6fe80b88d56",
		Status:    "Available",
		Metrics:   pm,
	}

	logEvent := logsapi.LogEvent{
		Time:         timestamp,
		Type:         "platform.report",
		StringRecord: "",
		Record:       logEventRecord,
	}

	event := NextEventResponse{
		Timestamp:          timestamp,
		EventType:          Invoke,
		DeadlineMs:         timestamp.UnixMilli() + 4584,
		RequestID:          "8476a536-e9f4-11e8-9739-2dfe598c3fcd",
		InvokedFunctionArn: "arn:aws:lambda:us-east-2:123456789012:function:custom-runtime",
		Tracing: Tracing{
			Type:  "None",
			Value: "None",
		},
	}

	desiredOutput := fmt.Sprintf(`{"metadata":{"service":{"agent":{"name":"aws-lambda-extension","version":"0.0.3"},"framework":{"name":"AWS Lambda","version":""},"language":{"name":"python","version":"3.9.8"},"runtime":{"name":"","version":""},"node":{}},"user":{},"process":{"pid":0},"system":{"container":{"id":""},"kubernetes":{"node":{},"pod":{}}},"cloud":{"provider":"","instance":{},"machine":{},"account":{},"project":{},"service":{}}}}
{"metricset":{"timestamp":%d,"transaction":{},"span":{},"tags":{"faas.coldstart":true,"faas.execution":"6f7f0961f83442118a7af6fe80b88d56","faas.id":"arn:aws:lambda:us-east-2:123456789012:function:custom-runtime"},"samples":{"aws.lambda.metrics.BilledDuration":{"value":183},"aws.lambda.metrics.ColdStartDuration":{"value":422.9700012207031},"aws.lambda.metrics.Duration":{"value":182.42999267578125},"aws.lambda.metrics.Timeout":{"value":5000},"aws.lambda.metrics.TotalMemory":{"value":134217728},"aws.lambda.metrics.UsedMemory":{"value":79691776},"system.memory.actual.free":{"value":54525952},"system.memory.total":{"value":134217728}}}}`, timestamp.UnixMicro())

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
		_, err = w.Write([]byte(`{"foo": "bar"}`))
		if err != nil {
			log.Println(err)
			t.Fail()
		}
	}))
	defer apmServer.Close()

	config := extensionConfig{
		apmServerUrl: apmServer.URL + "/",
	}

	mc := MetadataContainer{
		Metadata: &model.Metadata{},
	}
	mc.Metadata.Service = model.Service{
		Name:      os.Getenv("AWS_LAMBDA_FUNCTION_NAME"),
		Agent:     model.Agent{Name: "python", Version: "6.7.2"},
		Language:  model.Language{Name: "python", Version: "3.9.8"},
		Runtime:   model.Runtime{Name: os.Getenv("AWS_EXECUTION_ENV")},
		Framework: model.Framework{Name: "AWS Lambda"},
	}
	mc.Metadata.Process = model.Process{}
	mc.Metadata.System = model.System{}

	ProcessPlatformReport(apmServer.Client(), mc, &event, logEvent, &config)
}
