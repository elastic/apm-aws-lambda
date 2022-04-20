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
	"elastic/apm-lambda-extension/extension"
	"elastic/apm-lambda-extension/model"
	"encoding/json"
	"net/http"
)

func handleLogEventsRequest(out chan model.LogEvent) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		var logEvents []model.LogEvent
		if err := json.NewDecoder(r.Body).Decode(&logEvents); err != nil {
			extension.Log.Errorf("Error unmarshalling log events: %+v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		for idx := range logEvents {
			if logEvents[idx].Type == "" {
				extension.Log.Errorf("Error reading log event: %+v", logEvents[idx])
				w.WriteHeader(http.StatusInternalServerError)
				continue
			}
			out <- logEvents[idx]
		}
	}
}
