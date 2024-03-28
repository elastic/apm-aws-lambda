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
	"github.com/elastic/apm-aws-lambda/logsapi/model"
	"go.elastic.co/fastjson"
)

// ProcessFunctionLog processes the `function` log line from lambda logs API and returns
// a byte array containing the JSON body for the extracted log along with the timestamp.
// A non nil error is returned when marshaling of the log into JSON fails.
func ProcessFunctionLog(
	requestID string,
	invokedFnArn string,
	log LogEvent,
) ([]byte, error) {
	lc := model.LogContainer{
		Log: &model.LogLine{
			Timestamp: model.Time(log.Time),
			Message:   log.StringRecord,
		},
	}

	lc.Log.FAAS = &model.FAAS{
		ID:        invokedFnArn,
		Execution: requestID,
	}

	var jsonWriter fastjson.Writer
	if err := lc.MarshalFastJSON(&jsonWriter); err != nil {
		return nil, err
	}

	return jsonWriter.Bytes(), nil
}
