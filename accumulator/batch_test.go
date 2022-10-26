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

const metadata = `{"metadata":{"service":{"agent":{"name":"apm-lambda-extension","version":"1.1.0"},"framework":{"name":"AWS Lambda","version":""},"language":{"name":"python","version":"3.9.8"},"runtime":{"name":"","version":""},"node":{}},"user":{},"process":{"pid":0},"system":{"container":{"id":""},"kubernetes":{"node":{},"pod":{}}},"cloud":{"provider":"","instance":{},"machine":{},"account":{},"project":{},"service":{}}}}`

func TestAdd(t *testing.T) {
	t.Run("empty-without-metadata", func(t *testing.T) {
		b := NewBatch(1, time.Hour)
		assert.Error(t, b.AddLambdaData([]byte{}), ErrMetadataUnavailable)
	})
	t.Run("empty-with-metadata", func(t *testing.T) {
		b := NewBatch(1, time.Hour)
		b.RegisterInvocation("test", "arn", 500, time.Now())
		require.NoError(t, b.AddAgentData(APMData{Data: []byte(metadata)}))
		assert.NoError(t, b.AddLambdaData([]byte{}))
	})
	t.Run("full", func(t *testing.T) {
		b := NewBatch(1, time.Hour)
		b.RegisterInvocation("test", "arn", 500, time.Now())
		require.NoError(t, b.AddAgentData(APMData{Data: []byte(metadata)}))
		require.NoError(t, b.AddLambdaData([]byte{}))

		assert.ErrorIs(t, ErrBatchFull, b.AddLambdaData([]byte{}))
	})
}

func TestReset(t *testing.T) {
	b := NewBatch(1, time.Hour)
	b.RegisterInvocation("test", "arn", 500, time.Now())
	require.NoError(t, b.AddAgentData(APMData{Data: []byte(metadata)}))
	require.NoError(t, b.AddLambdaData([]byte{}))
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
		require.NoError(t, b.AddLambdaData([]byte{}))
	}

	require.Equal(t, 9, b.Count())
	assert.True(t, b.ShouldShip())
}

func TestShouldShip_ReasonAge(t *testing.T) {
	b := NewBatch(10, time.Second)
	b.RegisterInvocation("test", "arn", 500, time.Now())
	require.NoError(t, b.AddAgentData(APMData{Data: []byte(metadata)}))

	assert.False(t, b.ShouldShip())
	require.NoError(t, b.AddLambdaData([]byte{}))

	time.Sleep(time.Second + time.Millisecond)

	// Should be ready to send now
	require.Equal(t, 1, b.Count())
	assert.True(t, b.ShouldShip())
}
