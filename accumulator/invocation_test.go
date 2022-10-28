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

package accumulator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFinalizeAndEnrich_TxnExists(t *testing.T) {
	ts := time.Date(2022, time.October, 1, 1, 0, 0, 0, time.UTC)
	data := `{"transaction":{"id":"txn-id","trace_id":"trace-id","outcome":"success"}}`
	inc := &Invocation{
		Timestamp:     ts,
		DeadlineMs:    ts.Add(time.Minute).UnixMilli(),
		FunctionARN:   "test-fn-arn",
		TransactionID: "txn-id",
		agentData:     [][]byte{[]byte(data)},
	}

	inc.Finalize("success") // does nothing
	assert.Equal(t, 1, len(inc.agentData))
	assert.Equal(t, data, string(inc.agentData[0]))
}

func TestFinalizeAndEnrich_TxnNotFound(t *testing.T) {
	ts := time.Date(2022, time.October, 1, 1, 0, 0, 0, time.UTC)
	inc := &Invocation{
		Timestamp:     ts,
		DeadlineMs:    ts.Add(time.Minute).UnixMilli(),
		FunctionARN:   "test-fn-arn",
		TransactionID: "txn-id",
		TraceID:       "trace-id",
	}

	expected := `{"transaction":{"id":"txn-id","trace_id":"trace-id","outcome":"timeout"}}`
	inc.Finalize("timeout")
	assert.JSONEq(t, expected, string(inc.agentData[0]))
}

func BenchmarkCreateProxyTxn(b *testing.B) {
	ts := time.Date(2022, time.October, 1, 1, 0, 0, 0, time.UTC)
	inc := &Invocation{
		Timestamp:     ts,
		DeadlineMs:    ts.Add(time.Minute).UnixMilli(),
		FunctionARN:   "test-fn-arn",
		TransactionID: "txn-id",
		TraceID:       "trace-id",
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		inc.createProxyTxn("success")
	}
}
