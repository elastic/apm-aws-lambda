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

package model

import (
	"encoding/json"
	"time"
)

// EventType represents the type of logs in Lambda
type EventType string

// SubEventType is a Logs API sub event type
type SubEventType string

// LogEvent represents an event received from the Logs API
type LogEvent struct {
	Time         time.Time    `json:"time"`
	Type         SubEventType `json:"type"`
	StringRecord string
	Record       LogEventRecord
}

// LogEventRecord is a sub-object in a Logs API event
type LogEventRecord struct {
	RequestId string          `json:"requestId"`
	Status    string          `json:"status"`
	Metrics   PlatformMetrics `json:"metrics"`
}

type PlatformMetrics struct {
	DurationMs       float32 `json:"durationMs"`
	BilledDurationMs int32   `json:"billedDurationMs"`
	MemorySizeMB     int32   `json:"memorySizeMB"`
	MaxMemoryUsedMB  int32   `json:"maxMemoryUsedMB"`
	InitDurationMs   float32 `json:"initDurationMs"`
}

func (le *LogEvent) UnmarshalJSON(data []byte) error {
	b := struct {
		Time   time.Time       `json:"time"`
		Type   SubEventType    `json:"type"`
		Record json.RawMessage `json:"record"`
	}{}

	if err := json.Unmarshal(data, &b); err != nil {
		return err
	}
	le.Time = b.Time
	le.Type = b.Type

	if len(b.Record) > 0 && b.Record[0] == '{' {
		if err := json.Unmarshal(b.Record, &(le.Record)); err != nil {
			return err
		}
	} else {
		if err := json.Unmarshal(b.Record, &(le.StringRecord)); err != nil {
			return err
		}
	}
	return nil
}
