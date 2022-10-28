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
	"time"

	"github.com/tidwall/gjson"
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
	// Metadata stripped data from the agent. Each line is represented as
	// a seperate entry.
	agentData [][]byte
}

// Finalize searches the agent data for an invocation to find the root txn
// for the invocation. If root txn is not found then a new txn is created
// with the payload submitted by the agent.
func (inc *Invocation) Finalize(status string) {
	if inc.TransactionID == "" {
		return
	}
	if rootTxnIdx := inc.findRootTxn(); rootTxnIdx == -1 {
		inc.createProxyTxn(status)
	}
}

func (inc *Invocation) createProxyTxn(status string) int {
	var w fastjson.Writer
	w.RawString(`{"transaction":{"id":`)
	w.String(inc.TransactionID)
	w.RawString(`,"trace_id":`)
	w.String(inc.TraceID)
	w.RawString(`,"result":`)
	w.String(status)
	w.RawString("}}")
	inc.agentData = append(inc.agentData, w.Bytes())
	return len(inc.agentData) - 1
}

func (inc *Invocation) findRootTxn() int {
	for i := range inc.agentData {
		switch t := findEventType(inc.agentData[i]); string(t) {
		case "transaction":
			// Get transaction.id and check if it matches
			res := gjson.GetBytes(inc.agentData[i], "transaction.id")
			if res.Str != "" && inc.TransactionID == res.Str {
				return i
			}
		}
	}
	return -1
}

func findEventType(body []byte) []byte {
	var quote byte
	var key []byte
	for i, r := range body {
		if r == '"' || r == '\'' {
			quote = r
			key = body[i+1:]
			break
		}
	}
	end := bytes.IndexByte(key, quote)
	if end == -1 {
		return nil
	}
	return key[:end]
}
