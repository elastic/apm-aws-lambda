package extension

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"elastic/apm-lambda-extension/logsapi"
	"fmt"
	"gotest.tools/assert"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
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

	desiredOutput := fmt.Sprintf(`{"metadata":{"service":{"agent":{"name":"python","version":"6.7.2"},"framework":{"name":"AWS Lambda","version":""},"language":{"name":"python","version":"3.9.8"},"runtime":{"name":"","version":""}},"process":{},"system":{}}}
{"metricset":{"timestamp":%d,"transaction":{},"span":{},"samples":{"faas.metrics.duration.billed.ms":{"value":183},"faas.metrics.duration.init.ms":{"value":422.9700012207031},"faas.metrics.duration.measured.ms":{"value":182.42999267578125},"faas.metrics.memory.maxUsed.bytes":{"value":79691776},"faas.metrics.memory.total.bytes":{"value":134217728},"system.memory.actual.free":{"value":54525952},"system.memory.total":{"value":134217728}}}}`, timestamp.UnixMicro())

	apmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestBytes, err := getDecompressedBytesFromRequest(r)
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

	ProcessPlatformReport(apmServer.Client(), timestamp, pm, &config)
}

func getDecompressedBytesFromRequest(req *http.Request) ([]byte, error) {
	var rawBytes []byte
	if req.Body != nil {
		rawBytes, _ = ioutil.ReadAll(req.Body)
	}

	switch req.Header.Get("Content-Encoding") {
	case "deflate":
		reader := bytes.NewReader([]byte(rawBytes))
		zlibreader, err := zlib.NewReader(reader)
		if err != nil {
			return nil, fmt.Errorf("could not create zlib.NewReader: %v", err)
		}
		bodyBytes, err := ioutil.ReadAll(zlibreader)
		if err != nil {
			return nil, fmt.Errorf("could not read from zlib reader using ioutil.ReadAll: %v", err)
		}
		return bodyBytes, nil
	case "gzip":
		reader := bytes.NewReader([]byte(rawBytes))
		zlibreader, err := gzip.NewReader(reader)
		if err != nil {
			return nil, fmt.Errorf("could not create gzip.NewReader: %v", err)
		}
		bodyBytes, err := ioutil.ReadAll(zlibreader)
		if err != nil {
			return nil, fmt.Errorf("could not read from gzip reader using ioutil.ReadAll: %v", err)
		}
		return bodyBytes, nil
	default:
		return rawBytes, nil
	}
}
