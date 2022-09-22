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

package apmproxy

import "errors"

// ErrBatchFull signfies that the batch has reached full
// capacity and cannot accept more entires.
var ErrBatchFull = errors.New("batch is full")

// BatchData represents a batch of data without metadata
// that will be sent to APMServer. BatchData is not safe
// concurrent access.
type BatchData struct {
	agentData []APMData
	maxSize   int
}

// NewBatch creates a new BatchData which can accept a
// maximum number of entires as specified by the argument
func NewBatch(maxSize int) *BatchData {
	return &BatchData{
		maxSize:   maxSize,
		agentData: make([]APMData, 0, maxSize),
	}
}

// Add adds a new entry to the batch. Returns ErrBatchFull
// if batch has reached it's maximum size.
func (b *BatchData) Add(d APMData) error {
	if len(b.agentData) >= b.maxSize {
		return ErrBatchFull
	}

	b.agentData = append(b.agentData, d)
	return nil
}

// Size return the number of entries in batch.
func (b *BatchData) Size() int {
	return len(b.agentData)
}

// ShouldFlush indicates when a batch is ready for flush.
// A batch is marked as ready for flush once it reaches
// 90% of its max size.
func (b *BatchData) ShouldFlush() bool {
	return len(b.agentData) >= int(float32(b.maxSize)*0.9)
}

// Reset resets the batch to prepare for new set of data
func (b *BatchData) Reset() {
	b.agentData = nil
	b.agentData = make([]APMData, 0, b.maxSize)
}
