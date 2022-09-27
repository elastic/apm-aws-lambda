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

import (
	"bytes"
	"errors"
	"time"
)

var (
	// ErrBatchFull signfies that the batch has reached full capacity
	// and cannot accept more entires.
	ErrBatchFull = errors.New("batch is full")
	// ErrInvalidType is returned for any APMData that is not Lambda type
	ErrInvalidType = errors.New("only accepts lambda type data")
	// ErrInvalidEncoding is returned for any APMData that is encoded
	// with any encoding format
	ErrInvalidEncoding = errors.New("encoded data not supported")
)

var (
	maxSizeThreshold = 0.9
)

// BatchData represents a batch of data without metadata
// that will be sent to APMServer. BatchData is not safe
// concurrent access.
type BatchData struct {
	metadataBytes   int
	buf             bytes.Buffer
	count           int
	maxSize         int
	maxAgeInSeconds int
	age             int64
}

// NewBatch creates a new BatchData which can accept a
// maximum number of entires as specified by the argument
func NewBatch(maxSize int, maxAgeInSeconds int, metadata []byte) *BatchData {
	b := &BatchData{
		maxSize:         maxSize,
		maxAgeInSeconds: maxAgeInSeconds,
	}
	b.metadataBytes, _ = b.buf.Write(metadata)
	return b
}

// Add adds a new entry to the batch. Returns ErrBatchFull
// if batch has reached it's maximum size.
func (b *BatchData) Add(d APMData) error {
	if b.count >= b.maxSize {
		return ErrBatchFull
	}
	if d.Type != Lambda {
		return ErrInvalidType
	}
	if d.ContentEncoding != "" {
		return ErrInvalidEncoding
	}
	if err := b.buf.WriteByte('\n'); err != nil {
		return err
	}
	if _, err := b.buf.Write(d.Data); err != nil {
		return err
	}
	if b.count == 0 {
		// For first entry, set the age of the batch
		b.age = time.Now().Unix()
	}
	b.count++
	return nil
}

// Count return the number of APMData entries in batch.
func (b *BatchData) Count() int {
	return b.count
}

// ShouldShip indicates when a batch is ready for sending.
// A batch is marked as ready for flush when one of the
// below conditions is reached:
// 1. max size is greater than threshold (90% of maxSize)
// 2. batch is older than maturity age
func (b *BatchData) ShouldShip() bool {
	return b.count >= int(float64(b.maxSize)*maxSizeThreshold) ||
		(b.age > 0 && time.Now().Unix()-b.age > int64(b.maxAgeInSeconds))
}

// Reset resets the batch to prepare for new set of data
func (b *BatchData) Reset() {
	b.count, b.age = 0, 0
	b.buf.Truncate(b.metadataBytes)
}

// ToAPMData returns APMData with metadata and the accumulated batch
func (b *BatchData) ToAPMData() APMData {
	return APMData{
		Data: b.buf.Bytes(),
		Type: Lambda,
	}
}
