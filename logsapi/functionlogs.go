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
	"github.com/elastic/apm-aws-lambda/apmproxy"
	"go.elastic.co/apm/v2/model"
	"go.elastic.co/fastjson"
)

type logContainer struct {
	Log *logLine
}

type logLine struct {
	Timestamp model.Time
	Message   string
	FAAS      *faas
}

func (l *logLine) MarshalFastJSON(w *fastjson.Writer) error {
	var firstErr error
	w.RawString("{\"message\":")
	w.String(l.Message)
	w.RawString(",\"@timestamp\":")
	if err := l.Timestamp.MarshalFastJSON(w); err != nil && firstErr == nil {
		firstErr = err
	}
	if l.FAAS != nil {
		w.RawString(",\"faas\":")
		if err := l.FAAS.MarshalFastJSON(w); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	w.RawByte('}')
	return firstErr
}

// faas struct is a subset of go.elastic.co/apm/v2/model#FAAS
//
// The purpose of having a separate struct is to have a custom
// marshalling logic that is targeted for the faas fields
// available for function logs. For example: `coldstart` value
// cannot be inferred for function logs so this struct drops
// the field entirely.
type faas struct {
	// ID holds a unique identifier of the invoked serverless function.
	ID string `json:"id,omitempty"`
	// Execution holds the request ID of the function invocation.
	Execution string `json:"execution,omitempty"`
}

func (f *faas) MarshalFastJSON(w *fastjson.Writer) error {
	w.RawString("{\"id\":")
	w.String(f.ID)
	w.RawString(",\"execution\":")
	w.String(f.Execution)
	w.RawByte('}')
	return nil
}

func (lc logContainer) MarshalFastJSON(json *fastjson.Writer) error {
	json.RawString(`{"log":`)
	if err := lc.Log.MarshalFastJSON(json); err != nil {
		return err
	}
	json.RawByte('}')
	return nil
}

// ProcessFunctionLog consumes agent metadata and log event from Lambda
// logs API to create a payload for APM server.
func ProcessFunctionLog(
	requestID string,
	invokedFnArn string,
	log LogEvent,
) (apmproxy.APMData, error) {
	lc := logContainer{
		Log: &logLine{
			Timestamp: model.Time(log.Time),
			Message:   log.StringRecord,
		},
	}

	lc.Log.FAAS = &faas{
		ID:        invokedFnArn,
		Execution: requestID,
	}

	var jsonWriter fastjson.Writer
	if err := lc.MarshalFastJSON(&jsonWriter); err != nil {
		return apmproxy.APMData{}, err
	}

	return apmproxy.APMData{
		Data: jsonWriter.Bytes(),
	}, nil
}
