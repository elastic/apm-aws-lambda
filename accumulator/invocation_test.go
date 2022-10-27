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
	"github.com/stretchr/testify/require"
)

func TestFinalizeAndEnrich_TxnExists(t *testing.T) {
	ts := time.Date(2022, time.October, 1, 1, 0, 0, 0, time.UTC)
	inc := &Invocation{
		Timestamp:     ts,
		DeadlineMs:    ts.Add(time.Minute).UnixMilli(),
		FunctionARN:   "test-fn-arn",
		TransactionID: "test",
		agentData: [][]byte{
			[]byte(`{"transaction":{"id":"test"}}`),
		},
	}

	expected := `{"transaction":{"id":"test"},"faas":{"billed_duration":11,"coldstart":true,"coldstart_duration":2,"duration":11.1,"execution":"","id":"test-fn-arn","timeout":60000},"system":{"memory":{"actual":{"free":1048576},"total":2097152}}}`
	require.NoError(t, inc.FinalizeAndEnrich(11.1, 2.0, 11, 2, 1))
	assert.Equal(t, expected, string(inc.agentData[0]))
}

func TestFinalizeAndEnrich_TxnNotFound(t *testing.T) {
	ts := time.Date(2022, time.October, 1, 1, 0, 0, 0, time.UTC)
	inc := &Invocation{
		Timestamp:     ts,
		DeadlineMs:    ts.Add(time.Minute).UnixMilli(),
		FunctionARN:   "test-fn-arn",
		TransactionID: "txn-id",
		TraceID:       "trace-id",
		Status:        "timeout",
	}

	expected := `{"transaction":{"id":"txn-id","trace_id":"trace-id","outcome":"timeout"},"faas":{"billed_duration":11,"coldstart":true,"coldstart_duration":2,"duration":11.1,"execution":"","id":"test-fn-arn","timeout":60000},"system":{"memory":{"actual":{"free":1048576},"total":2097152}}}`
	require.NoError(t, inc.FinalizeAndEnrich(11.1, 2.0, 11, 2, 1))
	assert.JSONEq(t, expected, string(inc.agentData[0]))
}
