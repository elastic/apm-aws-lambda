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
	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

const metadata = `{"metadata":{"service":{"agent":{"name":"apm-lambda-extension","version":"1.1.0"},"framework":{"name":"AWS Lambda","version":""},"language":{"name":"python","version":"3.9.8"},"runtime":{"name":"","version":""},"node":{}},"user":{},"process":{"pid":0},"system":{"container":{"id":""},"kubernetes":{"node":{},"pod":{}}},"cloud":{"provider":"","instance":{},"machine":{},"account":{},"project":{},"service":{}}}}`

func TestAdd(t *testing.T) {
	t.Run("empty-without-metadata", func(t *testing.T) {
		b := NewBatch(1, time.Hour)
		assert.Error(t, b.AddLambdaData([]byte(`{"log":{}}`)), ErrMetadataUnavailable)
	})
	t.Run("empty-with-metadata", func(t *testing.T) {
		b := NewBatch(1, time.Hour)
		b.RegisterInvocation("test", "arn", 500, time.Now())
		require.NoError(t, b.AddAgentData(APMData{Data: []byte(metadata)}))
		assert.NoError(t, b.AddLambdaData([]byte(`{"log":{}}`)))
	})
	t.Run("full", func(t *testing.T) {
		b := NewBatch(1, time.Hour)
		b.RegisterInvocation("test", "arn", 500, time.Now())
		require.NoError(t, b.AddAgentData(APMData{Data: []byte(metadata)}))
		require.NoError(t, b.AddLambdaData([]byte(`{"log":{}}`)))

		assert.ErrorIs(t, ErrBatchFull, b.AddLambdaData([]byte(`{"log":{}}`)))
	})
}

func TestReset(t *testing.T) {
	b := NewBatch(1, time.Hour)
	b.RegisterInvocation("test", "arn", 500, time.Now())
	require.NoError(t, b.AddAgentData(APMData{Data: []byte(metadata)}))
	require.NoError(t, b.AddLambdaData([]byte(`{"log":{}}`)))
	require.Equal(t, 1, b.Count())
	b.Reset()

	assert.Equal(t, 0, b.Count())
	assert.True(t, b.age.IsZero())
}

func TestShouldShip_ReasonSize(t *testing.T) {
	b := NewBatch(10, time.Hour)
	b.RegisterInvocation("test", "arn", 500, time.Now())
	require.NoError(t, b.AddAgentData(APMData{Data: []byte(metadata)}))

	// Should flush at 90% full
	for i := 0; i < 9; i++ {
		assert.False(t, b.ShouldShip())
		require.NoError(t, b.AddLambdaData([]byte(`{"log":{}}`)))
	}

	require.Equal(t, 9, b.Count())
	assert.True(t, b.ShouldShip())
}

func TestShouldShip_ReasonAge(t *testing.T) {
	b := NewBatch(10, time.Second)
	b.RegisterInvocation("test", "arn", 500, time.Now())
	require.NoError(t, b.AddAgentData(APMData{Data: []byte(metadata)}))

	assert.False(t, b.ShouldShip())
	require.NoError(t, b.AddLambdaData([]byte(`{"log":{}}`)))

	time.Sleep(time.Second + time.Millisecond)

	// Should be ready to send now
	require.Equal(t, 1, b.Count())
	assert.True(t, b.ShouldShip())
}

func TestLifecycle(t *testing.T) {
	reqID := "test-req-id"
	fnARN := "test-fn-arn"
	txnID := "023d90ff77f13b9f"
	lambdaData := `{"log":{"message":"this is log"}}`
	txnData := fmt.Sprintf(`{"transaction":{"id":"%s"}}`, txnID)
	ts := time.Date(2022, time.October, 1, 1, 1, 1, 0, time.UTC)

	for _, tc := range []struct {
		name                    string
		agentInit               bool
		receiveAgentRootTxn     bool
		receiveLambdaLogRuntime bool
		expected                string
	}{
		{
			name:                    "without_agent_init_without_root_txn",
			agentInit:               false,
			receiveAgentRootTxn:     false,
			receiveLambdaLogRuntime: false,
			// Without agent init no proxy txn is created if root txn is not reported
			expected: fmt.Sprintf(
				"%s\n%s",
				metadata,
				lambdaData,
			),
		},
		{
			name:                    "without_agent_init_with_root_txn",
			agentInit:               false,
			receiveAgentRootTxn:     true,
			receiveLambdaLogRuntime: false,
			expected: fmt.Sprintf(
				"%s\n%s\n%s",
				metadata,
				generateCompleteTxn(t, txnData, "success", ""),
				lambdaData,
			),
		},
		{
			name:                    "with_agent_init_with_root_txn",
			agentInit:               true,
			receiveAgentRootTxn:     true,
			receiveLambdaLogRuntime: false,
			expected: fmt.Sprintf(
				"%s\n%s\n%s",
				metadata,
				generateCompleteTxn(t, txnData, "success", ""),
				lambdaData,
			),
		},
		{
			name:                    "with_agent_init_without_root_txn_with_runtimeDone",
			agentInit:               true,
			receiveAgentRootTxn:     false,
			receiveLambdaLogRuntime: true,
			// With agent init proxy txn is created if root txn is not reported.
			// Details in runtimeDone event is used to find the result of the txn.
			expected: fmt.Sprintf(
				"%s\n%s\n%s",
				metadata,
				lambdaData,
				generateCompleteTxn(t, txnData, "failure", "failure"),
			),
		},
		{
			name:                    "with_agent_init_without_root_txn",
			agentInit:               true,
			receiveAgentRootTxn:     false,
			receiveLambdaLogRuntime: false,
			// With agent init proxy txn is created if root txn is not reported.
			// If runtimeDone event is not available `timeout` is used as the
			// result of the transaction.
			expected: fmt.Sprintf(
				"%s\n%s\n%s",
				metadata,
				lambdaData,
				generateCompleteTxn(t, txnData, "timeout", "failure"),
			),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := NewBatch(100, time.Hour)
			// NEXT API response creates a new invocation cache
			b.RegisterInvocation(reqID, fnARN, 100, ts)
			// Agent creates and registers a partial transaction in the extn
			if tc.agentInit {
				require.NoError(t, b.OnAgentInit(txnID, []byte(txnData)))
			}
			// Agent sends a request with metadata
			require.NoError(t, b.AddAgentData(APMData{
				Data: []byte(metadata),
			}))
			if tc.receiveAgentRootTxn {
				require.NoError(t, b.AddAgentData(APMData{
					Data: []byte(fmt.Sprintf(
						"%s\n%s",
						metadata,
						generateCompleteTxn(t, txnData, "success", "")),
					),
				}))
			}
			// Lambda API receives a platform.Start event followed by
			// function events.
			require.NoError(t, b.AddLambdaData([]byte(lambdaData)))
			if tc.receiveLambdaLogRuntime {
				// Lambda API receives a platform.runtimeDone event
				require.NoError(t, b.OnLambdaLogRuntimeDone(reqID, "failure"))
			}
			// Instance shutdown
			require.NoError(t, b.OnShutdown("timeout"))
			assert.Equal(t, tc.expected, string(b.ToAPMData().Data))
		})
	}
}

func generateCompleteTxn(t *testing.T, src, result, outcome string) string {
	t.Helper()
	tmp, err := sjson.SetBytes([]byte(src), "transaction.result", result)
	assert.NoError(t, err)
	if outcome != "" {
		tmp, err = sjson.SetBytes(tmp, "transaction.outcome", outcome)
		assert.NoError(t, err)
	}
	return string(tmp)
}
