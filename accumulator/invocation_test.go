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

func TestFinalize(t *testing.T) {
	for _, tc := range []struct {
		name        string
		txnID       string
		traceID     string
		txnObserved bool
		output      string
	}{
		{
			name: "no_txn_registered",
		},
		{
			name:        "txn_registered_observed",
			txnID:       "test-txn-id",
			traceID:     "test-trace-id",
			txnObserved: true,
		},
		{
			name:    "txn_registered_not_observed",
			txnID:   "test-txn-id",
			traceID: "test-trace-id",
			output:  `{"transaction":{"id":"test-txn-id","trace_id":"test-trace-id","result":"success"}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts := time.Date(2022, time.October, 1, 1, 0, 0, 0, time.UTC)
			inc := &Invocation{
				Timestamp:           ts,
				DeadlineMs:          ts.Add(time.Minute).UnixMilli(),
				FunctionARN:         "test-fn-arn",
				TransactionID:       tc.txnID,
				TraceID:             tc.traceID,
				TransactionObserved: tc.txnObserved,
			}
			if len(tc.output) > 0 {
				assert.JSONEq(t, tc.output, string(inc.Finalize("success")))
			} else {
				assert.Nil(t, inc.Finalize("success"))
			}
		})
	}
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
