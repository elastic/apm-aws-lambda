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
	"math"
	"time"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Invocation holds data for each function invocation and finalizes
// the data when `platform.report` type log is received for the
// specific invocation identified by request ID.
type Invocation struct {
	RequestID string
	Timestamp time.Time
	// DeadlineMs is the function execution deadline.
	DeadlineMs int64
	// FunctionARN requested. Can be different in each invoke that
	// executes the same version.
	FunctionARN string
	// TransactionID is the ID generated for a transaction for the
	// current invocation. It is populated by the request from agent.
	TransactionID string
	// AgentPayload for creating transaction.
	AgentPayload []byte
	// Status of the invocation, is available in the platform.runtimeDone
	// log from the Lambda API.
	Status string
	// Metadata stripped data from the agent. Each line is represented as
	// a seperate entry.
	agentData [][]byte
}

// Finalize searches the agent data for an invocation to find the root txn
// for the invocation. If root txn is not found then a new txn is created
// with the payload submitted by the agent.
func (inc *Invocation) Finalize() error {
	if inc.TransactionID == "" {
		return nil
	}
	if rootTxnIdx := inc.findRootTxn(); rootTxnIdx == -1 {
		// Create a new txn if not found
		txn, err := sjson.SetRawBytes(nil, "transaction", inc.AgentPayload)
		if err != nil {
			return err
		}
		inc.agentData = append(inc.agentData, txn)
	}
	return nil
}

// FinalizeAndEnrich finalizes the invocation and uses the data from
// platform.report metrics to enrich the root txn.
func (inc *Invocation) FinalizeAndEnrich(
	durationMs, initMs float32,
	billDurationMs, memMB, maxMemMB int32,
) error {
	if inc.TransactionID == "" {
		return nil
	}
	rootTxnIdx := inc.findRootTxn()
	if rootTxnIdx == -1 {
		// Create a new txn if not found
		txn, err := sjson.SetRawBytes(nil, "transaction", inc.AgentPayload)
		if err != nil {
			return err
		}
		inc.agentData = append(inc.agentData, txn)
		rootTxnIdx = len(inc.agentData) - 1
	}
	// Enrich the transaction with the platform metrics
	txn, err := sjson.SetBytes(inc.agentData[rootTxnIdx], "faas", map[string]interface{}{
		"execution":          inc.RequestID,
		"id":                 inc.FunctionARN,
		"timeout":            math.Ceil(float64(inc.DeadlineMs-inc.Timestamp.UnixMilli())/1e3) * 1e3,
		"coldstart":          initMs > 0,
		"duration":           durationMs,
		"billed_duration":    billDurationMs,
		"coldstart_duration": initMs,
	})
	if err != nil {
		return err
	}
	txn, err = sjson.SetBytes(txn, "system.memory", map[string]interface{}{
		"total": float64(memMB) * float64(1024*1024),
		"actual": map[string]float64{
			"free": float64(memMB-maxMemMB) * float64(1024*1024),
		},
	})
	if err != nil {
		return err
	}
	inc.agentData[rootTxnIdx] = txn
	return nil
}

func (inc *Invocation) findRootTxn() int {
	for i := range inc.agentData {
		switch t := findEventType(inc.agentData[i]); string(t) {
		case "transaction":
			// Get transaction.id and check if it matches
			var res gjson.Result
			res = gjson.GetBytes(inc.agentData[i], "transaction.id")
			if inc.TransactionID == res.Str {
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
