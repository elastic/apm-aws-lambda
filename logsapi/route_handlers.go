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

package logsapi

import (
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"
)

func handleLogEventsRequest(logger *zap.SugaredLogger, logsChannel chan LogEvent) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var logEvents []LogEvent
		if err := json.NewDecoder(r.Body).Decode(&logEvents); err != nil {
			logger.Errorf("Error unmarshalling log events: %+v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		for idx := range logEvents {
			if logEvents[idx].Type == "" {
				logger.Errorf("Error reading log event: %+v", logEvents[idx])
				w.WriteHeader(http.StatusInternalServerError)
				continue
			}
			select {
			case logsChannel <- logEvents[idx]:
			case <-r.Context().Done():
				logger.Warnf("Failed to enqueue event, signaling lambda to retry")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
	}
}

func (le *LogEvent) UnmarshalJSON(data []byte) error {
	b := struct {
		Time   time.Time       `json:"time"`
		Type   LogEventType    `json:"type"`
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
