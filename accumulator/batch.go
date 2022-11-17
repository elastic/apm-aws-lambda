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
	"bytes"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/tidwall/gjson"
)

var (
	// ErrMetadataUnavailable is returned when a lambda data is added to
	// the batch without metadata being set.
	ErrMetadataUnavailable = errors.New("metadata is not yet available")
	// ErrBatchFull signfies that the batch has reached full capacity
	// and cannot accept more entries.
	ErrBatchFull = errors.New("batch is full")
	// ErrInvalidEncoding is returned for any APMData that is encoded
	// with any encoding format
	ErrInvalidEncoding = errors.New("encoded data not supported")
)

var (
	maxSizeThreshold     = 0.9
	zeroTime             = time.Time{}
	newLineSep           = []byte("\n")
	transactionKey       = []byte("transaction")
	waitingInvocationKey = "waiting"
)

// Batch manages the data that needs to be shipped to APM Server. It holds
// all the invocations that have not yet been shipped to the APM Server and
// is responsible for correlating the invocation with the APM data collected
// from all sources (logs API & APM Agents). As the batch gets the required
// data it marks the data ready for shipping to APM Server.
type Batch struct {
	mu sync.RWMutex
	// metadataBytes is the size of the metadata in bytes
	metadataBytes int
	// buf holds data that is ready to be shipped to APM-Server
	buf bytes.Buffer
	// invocations holds the data for a specific invocation with
	// request ID as the key.
	invocations                 map[string]*Invocation
	count                       int
	age                         time.Time
	maxSize                     int
	maxAge                      time.Duration
	currentlyExecutingRequestID string
}

// NewBatch creates a new BatchData which can accept a
// maximum number of entries as specified by the arguments.
func NewBatch(maxSize int, maxAge time.Duration) *Batch {
	return &Batch{
		invocations: make(map[string]*Invocation),
		maxSize:     maxSize,
		maxAge:      maxAge,
	}
}

// RegisterInvocation registers a new function invocation against its request
// ID. It also updates the caches for currently executing request ID.
func (b *Batch) RegisterInvocation(
	requestID, functionARN string,
	deadlineMs int64,
	timestamp time.Time,
) {
	b.mu.Lock()
	defer b.mu.Unlock()
	inc := &Invocation{
		RequestID:   requestID,
		FunctionARN: functionARN,
		DeadlineMs:  deadlineMs,
		Timestamp:   timestamp,
	}
	if w, ok := b.invocations[waitingInvocationKey]; ok {
		// If the agent payload is cached before the invocation is registered
		// then associate the agent payload with the invocation.
		inc.AgentPayload = w.AgentPayload
		inc.TransactionID = w.TransactionID
		delete(b.invocations, waitingInvocationKey)
	}
	b.invocations[requestID] = inc
	b.currentlyExecutingRequestID = requestID
}

// OnAgentInit caches the transactionID and the payload for the currently
// executing invocation as reported by the agent. The traceID and transactionID
// will be used to create a new transaction in an event the actual transaction
// is not reported by the agent due to unexpected termination. If the current
// invocation is not registered yet then the payload is cached temporarily and
// associated with the invocation when it is registered.
func (b *Batch) OnAgentInit(transactionID string, payload []byte) error {
	if !isTransactionEvent(payload) {
		return errors.New("invalid payload")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	i, ok := b.invocations[b.currentlyExecutingRequestID]
	if !ok {
		// It is possible that the invocation is registered at a later time
		b.invocations[waitingInvocationKey] = &Invocation{
			TransactionID: transactionID,
			AgentPayload:  payload,
		}
		return nil
	}
	i.TransactionID, i.AgentPayload = transactionID, payload
	return nil
}

// AddAgentData adds a data received from agent. For a specific invocation
// agent data is always received in the same invocation. All the events
// extracted from the payload are added to the batch even though the batch
// might exceed the max size limit, however, if the batch is already full
// before adding any events then ErrBatchFull is returned.
func (b *Batch) AddAgentData(apmData APMData) error {
	if len(apmData.Data) == 0 {
		return nil
	}
	raw, err := GetUncompressedBytes(apmData.Data, apmData.ContentEncoding)
	if err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.count >= b.maxSize {
		return ErrBatchFull
	}
	if b.currentlyExecutingRequestID == "" {
		return fmt.Errorf("lifecycle error, currently executing requestID is not set")
	}
	inc, ok := b.invocations[b.currentlyExecutingRequestID]
	if !ok {
		return fmt.Errorf("invocation for current requestID %s does not exist", b.currentlyExecutingRequestID)
	}

	// A request body can either be empty or have a ndjson content with
	// first line being metadata.
	data, after, _ := bytes.Cut(raw, newLineSep)
	if b.metadataBytes == 0 {
		b.metadataBytes, _ = b.buf.Write(data)
	}
	for {
		data, after, _ = bytes.Cut(after, newLineSep)
		if inc.NeedProxyTransaction() && isTransactionEvent(data) {
			res := gjson.GetBytes(data, "transaction.id")
			if res.Str != "" && inc.TransactionID == res.Str {
				inc.TransactionObserved = true
			}
		}
		if err := b.addData(data); err != nil {
			return err
		}
		if len(after) == 0 {
			break
		}
	}
	return nil
}

// OnLambdaLogRuntimeDone prepares the data for the invocation to be shipped
// to APM Server. It accepts requestID and status of the invocation both of
// which can be retrieved after parsing `platform.runtimeDone` event.
func (b *Batch) OnLambdaLogRuntimeDone(reqID, status string, time time.Time) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.finalizeInvocation(reqID, status, time)
}

// OnShutdown flushes the data for shipping to APM Server by finalizing all
// the invocation in the batch. If we haven't received a platform.runtimeDone
// event for an invocation so far we won't be able to recieve it in time thus
// the status needs to be guessed based on the available information.
func (b *Batch) OnShutdown(status string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, inc := range b.invocations {
		// Assume that the transaction took all the function time.
		// TODO: @lahsivjar Is it possible to tweak the extension lifecycle in
		// a way that we receive the platform.report metric for a invocation
		// consistently and enrich the metrics with reported values?
		time := time.Unix(0, inc.DeadlineMs*int64(time.Millisecond))
		if err := b.finalizeInvocation(inc.RequestID, status, time); err != nil {
			return err
		}
	}
	return nil
}

// AddLambdaData adds a new entry to the batch. Returns ErrBatchFull
// if batch has reached its maximum size.
func (b *Batch) AddLambdaData(d []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.count >= b.maxSize {
		return ErrBatchFull
	}
	return b.addData(d)
}

// Count return the number of APMData entries in batch.
func (b *Batch) Count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.count
}

// ShouldShip indicates when a batch is ready for sending.
// A batch is marked as ready for flush when one of the
// below conditions is reached:
// 1. max size is greater than threshold (90% of maxSize)
// 2. batch is older than maturity age
func (b *Batch) ShouldShip() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return (b.count >= int(float64(b.maxSize)*maxSizeThreshold)) ||
		(!b.age.IsZero() && time.Since(b.age) > b.maxAge)
}

// Reset resets the batch to prepare for new set of data
func (b *Batch) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.count, b.age = 0, zeroTime
	b.buf.Truncate(b.metadataBytes)
}

// ToAPMData returns APMData with metadata and the accumulated batch
func (b *Batch) ToAPMData() APMData {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return APMData{
		Data: b.buf.Bytes(),
	}
}

func (b *Batch) finalizeInvocation(reqID, status string, time time.Time) error {
	inc, ok := b.invocations[reqID]
	if !ok {
		return fmt.Errorf("invocation for requestID %s does not exist", reqID)
	}
	defer delete(b.invocations, reqID)
	proxyTxn, err := inc.Finalize(status, time)
	if err != nil {
		return err
	}
	return b.addData(proxyTxn)
}

func (b *Batch) addData(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if b.metadataBytes == 0 {
		return ErrMetadataUnavailable
	}
	if err := b.buf.WriteByte('\n'); err != nil {
		return err
	}
	if _, err := b.buf.Write(data); err != nil {
		return err
	}
	if b.count == 0 {
		// For first entry, set the age of the batch
		b.age = time.Now()
	}
	b.count++
	return nil
}

func isTransactionEvent(body []byte) bool {
	var key []byte
	for i, r := range body {
		if r == '"' || r == '\'' {
			key = body[i+1:]
			break
		}
	}
	if len(key) < len(transactionKey) {
		return false
	}
	for i := 0; i < len(transactionKey); i++ {
		if transactionKey[i] != key[i] {
			return false
		}
	}
	return true
}
