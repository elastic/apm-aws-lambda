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
	"elastic/apm-lambda-extension/extension"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListenOnAddressWithEnvVariable(t *testing.T) {
	extension.Log = extension.InitLogger()

	os.Setenv("ELASTIC_APM_LAMBDA_LOGS_LISTENER_ADDRESS", "example:3456")

	address := ListenOnAddress()
	t.Logf("%v", address)

	if address != "example:3456" {
		t.Log("Address was not taken from ENV variable correctly")
		t.Fail()
	}
}

func TestListenOnAddressDefault(t *testing.T) {
	extension.Log = extension.InitLogger()

	os.Unsetenv("ELASTIC_APM_LAMBDA_LOGS_LISTENER_ADDRESS")
	address := ListenOnAddress()
	t.Logf("%v", address)

	if address != "sandbox:1234" {
		t.Log("Default address was not used")
		t.Fail()
	}
}

func Test_unmarshalRuntimeDoneRecordObject(t *testing.T) {
	extension.Log = extension.InitLogger()

	jsonBytes := []byte(`
	{
		"time": "2021-10-20T08:13:03.278Z",
		"type": "platform.runtimeDone",
		"record": {
			"requestId": "61c0fdeb-f013-4f2a-b627-56278f5666b8"
		}
	}
	`)

	var le LogEvent
	err := json.Unmarshal(jsonBytes, &le)
	if err != nil {
		t.Fail()
	}

	err = le.unmarshalRecord()
	if err != nil {
		t.Fail()
	}

	record := LogEventRecord{RequestId: "61c0fdeb-f013-4f2a-b627-56278f5666b8"}
	assert.Equal(t, record, le.Record)
}

func Test_unmarshalFaultRecordString(t *testing.T) {
	extension.Log = extension.InitLogger()

	jsonBytes := []byte(`
	{
		"time": "2021-10-20T08:13:03.278Z",
		"type": "platform.fault",
		"record": "Unknown application error occurred"
	}
	`)

	var le LogEvent
	err := json.Unmarshal(jsonBytes, &le)
	if err != nil {
		t.Fail()
	}

	err = le.unmarshalRecord()
	if err != nil {
		t.Fail()
	}

	assert.Equal(t, "\"Unknown application error occurred\"", string(le.RawRecord))
}

func Test_unmarshalRecordError(t *testing.T) {
	extension.Log = extension.InitLogger()

	jsonBytes := []byte(`
	{
		"time": "2021-10-20T08:13:03.278Z",
		"type": "platform.runtimeDone",
		"record": "Unknown application error occurred"
	}
	`)

	var le LogEvent
	err := json.Unmarshal(jsonBytes, &le)
	if err != nil {
		t.Fail()
	}

	err = le.unmarshalRecord()
	assert.Error(t, err)
}
