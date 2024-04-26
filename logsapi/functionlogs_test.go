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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessFunctionLog(t *testing.T) {
	event := LogEvent{
		Time:         time.Date(2022, 11, 12, 0, 0, 0, 0, time.UTC),
		Type:         FunctionLog,
		StringRecord: "ERROR encountered. Stack trace:my-function (line 10)",
	}
	reqID := "8476a536-e9f4-11e8-9739-2dfe598c3fcd"
	invokedFnArn := "arn:aws:lambda:us-east-2:123456789012:function:custom-runtime"
	expectedData := fmt.Sprintf(
		"{\"log\":{\"@timestamp\":%d,\"message\":\"%s\",\"faas\":{\"execution\":\"%s\",\"id\":\"%s\"}}}",
		event.Time.UnixNano()/int64(time.Microsecond),
		event.StringRecord,
		reqID,
		invokedFnArn,
	)

	data, err := ProcessFunctionLog(reqID, invokedFnArn, event)

	require.NoError(t, err)
	assert.Equal(t, expectedData, string(data))
}
