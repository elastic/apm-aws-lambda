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
	"time"

	"go.elastic.co/fastjson"
)

// Invocation holds data for each function invocation and finalizes
// the data when `platform.report` type log is received for the
// specific invocation identified by request ID.
type Invocation struct {
	// RequestID is the id to identify the invocation.
	RequestID string
	// Timestamp is the time of the invocation.
	Timestamp time.Time
	// DeadlineMs is the function execution deadline.
	DeadlineMs int64
	// FunctionARN requested. Can be different in each invoke that
	// executes the same version.
	FunctionARN string
	// TransactionID is the ID generated for a transaction for the
	// current invocation. It is populated by the request from agent.
	TransactionID string
	// TraceID is the ID generated for a trace for the current invocation.
	// It is populated by the request from agent.
	TraceID string
	// TransactionObserved is true if the root transaction ID for the
	// invocation is observed by the extension.
	TransactionObserved bool
}

func (inc *Invocation) NeedProxyTransaction() bool {
	return inc.TransactionID != "" && !inc.TransactionObserved
}

// Finalize creates a proxy transaction for an invocation if required.
// A proxy transaction will be required to be created if the agent has
// registered a transaction for the invocation but has not sent the
// corresponding transaction to the extension.
func (inc *Invocation) Finalize(status string) []byte {
	if !inc.NeedProxyTransaction() {
		return nil
	}
	return inc.createProxyTxn(status)
}

func (inc *Invocation) createProxyTxn(status string) []byte {
	var w fastjson.Writer
	w.RawString(`{"transaction":{"id":`)
	w.String(inc.TransactionID)
	w.RawString(`,"trace_id":`)
	w.String(inc.TraceID)
	w.RawString(`,"result":`)
	w.String(status)
	w.RawString("}}")
	return w.Bytes()
}
