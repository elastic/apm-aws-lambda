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
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogEventUnmarshalReport(t *testing.T) {
	le := new(LogEvent)
	reportJSON := []byte(`{
		    "time": "2020-08-20T12:31:32.123Z",
		    "type": "platform.report",
		    "record": {"requestId": "6f7f0961f83442118a7af6fe80b88d56",
			"metrics": {"durationMs": 101.51,
			    "billedDurationMs": 300,
			    "memorySizeMB": 512,
			    "maxMemoryUsedMB": 33,
			    "initDurationMs": 116.67
			}
		    }
		}`)

	err := le.UnmarshalJSON(reportJSON)
	require.NoError(t, err)
	assert.Equal(t, LogEventType("platform.report"), le.Type)
	assert.Equal(t, "2020-08-20T12:31:32.123Z", le.Time.Format(time.RFC3339Nano))
	rec := LogEventRecord{
		RequestID: "6f7f0961f83442118a7af6fe80b88d56",
		Status:    "", // no status was given in sample json
		Metrics: PlatformMetrics{
			DurationMs:       101.51,
			BilledDurationMs: 300,
			MemorySizeMB:     512,
			MaxMemoryUsedMB:  33,
			InitDurationMs:   116.67,
		},
	}
	assert.Equal(t, rec, le.Record)
}

func TestLogEventUnmarshalFault(t *testing.T) {
	le := new(LogEvent)
	reportJSON := []byte(` {
		    "time": "2020-08-20T12:31:32.123Z",
		    "type": "platform.fault",
		    "record": "RequestId: d783b35e-a91d-4251-af17-035953428a2c Process exited before completing request"
		}`)

	err := le.UnmarshalJSON(reportJSON)
	require.NoError(t, err)
	assert.Equal(t, LogEventType("platform.fault"), le.Type)
	assert.Equal(t, "2020-08-20T12:31:32.123Z", le.Time.Format(time.RFC3339Nano))
	rec := "RequestId: d783b35e-a91d-4251-af17-035953428a2c Process exited before completing request"
	assert.Equal(t, rec, le.StringRecord)
}

func Test_unmarshalRuntimeDoneRecordObject(t *testing.T) {
	le := new(LogEvent)
	jsonBytes := []byte(`
	{
		"time": "2021-02-04T20:00:05.123Z",
		"type": "platform.runtimeDone",
		"record": {
		   "requestId":"6f7f0961f83442118a7af6fe80b88",
		   "status": "success"
		}
	}
	`)

	err := le.UnmarshalJSON(jsonBytes)
	require.NoError(t, err)
	assert.Equal(t, LogEventType("platform.runtimeDone"), le.Type)
	assert.Equal(t, "2021-02-04T20:00:05.123Z", le.Time.Format(time.RFC3339Nano))
	rec := LogEventRecord{
		RequestID: "6f7f0961f83442118a7af6fe80b88",
		Status:    "success",
	}
	assert.Equal(t, rec, le.Record)
}

func Test_unmarshalRuntimeDoneRecordString(t *testing.T) {
	le := new(LogEvent)
	jsonBytes := []byte(`
	{
		"time": "2021-02-04T20:00:05.123Z",
		"type": "platform.runtimeDone",
		"record": "Unknown application error occurred"
	}
	`)

	err := le.UnmarshalJSON(jsonBytes)
	require.NoError(t, err)
	assert.Equal(t, LogEventType("platform.runtimeDone"), le.Type)
	assert.Equal(t, "2021-02-04T20:00:05.123Z", le.Time.Format(time.RFC3339Nano))
	assert.Equal(t, "Unknown application error occurred", le.StringRecord)
}

func Test_unmarshalRuntimeDoneFaultRecordString(t *testing.T) {
	jsonBytes := []byte(`
		{
			"time": "2021-02-04T20:00:05.123Z",
			"type": "platform.fault",
			"record": "Unknown application error occurred"
		}
	`)

	var le LogEvent
	if err := json.Unmarshal(jsonBytes, &le); err != nil {
		t.Fail()
	}

	timeValue, _ := time.Parse(time.RFC3339, "2021-02-04T20:00:05.123Z")
	assert.Equal(t, timeValue, le.Time)
	assert.Equal(t, PlatformFault, le.Type)
	assert.Equal(t, "Unknown application error occurred", le.StringRecord)
}
