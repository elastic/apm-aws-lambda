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
	"io/ioutil"
	"log"
	"net/http"

	"github.com/pkg/errors"
)

func handleLogEventsRequest(out chan LogEvent) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("Error reading body of Logs API request: %+v", err)
			return
		}

		var logEvents []LogEvent
		err = json.Unmarshal(body, &logEvents)
		if err != nil {
			log.Println("Error unmarshalling log event batch:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		for idx := range logEvents {
			err = logEvents[idx].unmarshalRecord()
			if err != nil {
				log.Printf("Error unmarshalling log event: %+v", err)
				w.WriteHeader(http.StatusInternalServerError)
				continue
			}
			out <- logEvents[idx]
		}
	}
}

func (le *LogEvent) unmarshalRecord() error {
	if SubEventType(le.Type) != Fault {
		record := LogEventRecord{}
		err := json.Unmarshal([]byte(le.RawRecord), &record)
		if err != nil {
			return errors.New("Could not unmarshal log event raw record into record")
		}
		le.Record = record
	}
	return nil
}
