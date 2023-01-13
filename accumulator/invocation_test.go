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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCreateProxyTransaction(t *testing.T) {
	txnDur := time.Second
	for _, tc := range []struct {
		name              string
		txnID             string
		payload           string
		txnObserved       bool
		runtimeDoneStatus string
		output            string
	}{
		{
			name: "no_txn_registered",
		},
		{
			name:              "txn_registered_observed",
			txnID:             "test-txn-id",
			payload:           `{"transaction":{"id":"test-txn-id","trace_id":"test-trace-id","duration":-1}}`,
			txnObserved:       true,
			runtimeDoneStatus: "success",
		},
		{
			name:              "txn_registered_not_observed_runtime_failure",
			txnID:             "test-txn-id",
			payload:           `{"transaction":{"id":"test-txn-id","trace_id":"test-trace-id","duration":-1}}`,
			txnObserved:       false,
			runtimeDoneStatus: "failure",
			output: fmt.Sprintf(
				`{"transaction":{"id":"test-txn-id","trace_id":"test-trace-id","result":"failure","outcome":"failure","duration":%d}}`,
				txnDur.Milliseconds(),
			),
		},
		{
			name:              "txn_registered_not_observed_runtime_timeout",
			txnID:             "test-txn-id",
			payload:           `{"transaction":{"id":"test-txn-id","trace_id":"test-trace-id","duration":-1}}`,
			txnObserved:       false,
			runtimeDoneStatus: "timeout",
			output: fmt.Sprintf(
				`{"transaction":{"id":"test-txn-id","trace_id":"test-trace-id","result":"timeout","outcome":"failure","duration":%d}}`,
				txnDur.Milliseconds(),
			),
		},
		{
			name:              "txn_registered_not_observed_runtime_success",
			txnID:             "test-txn-id",
			payload:           `{"transaction":{"id":"test-txn-id","trace_id":"test-trace-id","duration":-1}}`,
			txnObserved:       false,
			runtimeDoneStatus: "success",
			output: fmt.Sprintf(
				`{"transaction":{"id":"test-txn-id","trace_id":"test-trace-id","result":"success","duration":%d}}`,
				txnDur.Milliseconds(),
			),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts := time.Date(2022, time.October, 1, 1, 0, 0, 0, time.UTC)
			inc := &Invocation{
				Timestamp:           ts,
				DeadlineMs:          ts.Add(time.Minute).UnixMilli(),
				FunctionARN:         "test-fn-arn",
				TransactionID:       tc.txnID,
				AgentPayload:        []byte(tc.payload),
				TransactionObserved: tc.txnObserved,
			}
			result, err := inc.CreateProxyTxn(tc.runtimeDoneStatus, ts.Add(txnDur))
			assert.Nil(t, err)
			if len(tc.output) > 0 {
				assert.JSONEq(t, tc.output, string(result))
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

func BenchmarkCreateProxyTxn(b *testing.B) {
	ts := time.Date(2022, time.October, 1, 1, 0, 0, 0, time.UTC)
	txnDur := ts.Add(time.Second)
	inc := &Invocation{
		Timestamp:     ts,
		DeadlineMs:    ts.Add(time.Minute).UnixMilli(),
		FunctionARN:   "test-fn-arn",
		TransactionID: "txn-id",
		AgentPayload:  []byte(`{"transaction":{"id":"txn-id"}}`),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := inc.CreateProxyTxn("success", txnDur)
		if err != nil {
			b.Fail()
		}
	}
}
