// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
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
)

func Test_unmarshalRuntimeDoneRecordObject(t *testing.T) {
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

	var le LogEvent
	err := json.Unmarshal(jsonBytes, &le)
	if err != nil {
		t.Fail()
	}

	timeValue, _ := time.Parse(time.RFC3339, "2021-02-04T20:00:05.123Z")
	assert.Equal(t, timeValue, le.Time)
	assert.Equal(t, RuntimeDone, le.Type)
	assert.Equal(t, "6f7f0961f83442118a7af6fe80b88", le.Record.RequestId)
	assert.Equal(t, "success", le.Record.Status)
}

func Test_unmarshalRuntimeDoneRecordString(t *testing.T) {
	jsonBytes := []byte(`
		{
			"time": "2021-02-04T20:00:05.123Z",
			"type": "platform.runtimeDone",
			"record": "Unknown application error occurred"
		}
	`)

	var le LogEvent
	err := json.Unmarshal(jsonBytes, &le)
	if err != nil {
		t.Fail()
	}

	timeValue, _ := time.Parse(time.RFC3339, "2021-02-04T20:00:05.123Z")
	assert.Equal(t, timeValue, le.Time)
	assert.Equal(t, RuntimeDone, le.Type)
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
	err := json.Unmarshal(jsonBytes, &le)
	if err != nil {
		t.Fail()
	}

	timeValue, _ := time.Parse(time.RFC3339, "2021-02-04T20:00:05.123Z")
	assert.Equal(t, timeValue, le.Time)
	assert.Equal(t, Fault, le.Type)
	assert.Equal(t, "Unknown application error occurred", le.StringRecord)
}
