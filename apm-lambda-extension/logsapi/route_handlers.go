// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
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
	"log"
	"net/http"
	"time"
)

func handleLogEventsRequest(out chan LogEvent) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		var logEvents []LogEvent
		if err := json.NewDecoder(r.Body).Decode(&logEvents); err != nil {
			log.Printf("Error unmarshalling log events: %+v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		for idx := range logEvents {
			if logEvents[idx].Type == "" {
				log.Printf("Error unmarshalling log event: %+v", logEvents[idx])
				w.WriteHeader(http.StatusInternalServerError)
				continue
			}
			out <- logEvents[idx]
		}
	}
}

func (le *LogEvent) UnmarshalJSON(data []byte) error {
	var temp map[string]interface{}
	err := json.Unmarshal(data, &temp)
	if err != nil {
		return err
	}

	for k, v := range temp {
		switch k {
		case "time":
			le.Time, err = time.Parse(time.RFC3339, v.(string))
			if err != nil {
				return err
			}
		case "type":
			le.Type = SubEventType(v.(string))
		case "record":
			rec, ok := v.(map[string]interface{})
			if ok {
				for m, n := range rec {
					switch m {
					case "requestId":
						le.Record.RequestId = n.(string)
					case "status":
						le.Record.Status = n.(string)
					}
				}
			} else {
				le.StringRecord = v.(string)
			}
		}
	}
	return nil
}
