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
	"github.com/elastic/apm-aws-lambda/extension"
	"go.elastic.co/apm/v2/model"
	"go.elastic.co/fastjson"
)

type LogContainer struct {
	Log *logLine
}

type logLine struct {
	Timestamp model.Time
	Message   string
	FAAS      *model.FAAS
}

func (l *logLine) MarshalFastJSON(w *fastjson.Writer) error {
	var firstErr error
	w.RawByte('{')
	w.RawString("\"message\":")
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

func (lc LogContainer) MarshalFastJSON(json *fastjson.Writer) error {
	json.RawString(`{"log":`)
	if err := lc.Log.MarshalFastJSON(json); err != nil {
		return err
	}
	json.RawString(`}`)
	return nil
}

// ProcessFunctionLog consumes extension event, agent metadata and log
// event from Lambda logs API to create a payload for APM server
func ProcessFunctionLog(
	metadataContainer *apmproxy.MetadataContainer,
	functionData *extension.NextEventResponse,
	log LogEvent,
) (apmproxy.AgentData, error) {
	lc := LogContainer{
		Log: &logLine{
			Timestamp: model.Time(log.Time),
			Message:   log.StringRecord,
		},
	}

	if functionData != nil {
		// FaaS Fields
		lc.Log.FAAS = &model.FAAS{
			Execution: functionData.RequestID,
			ID:        functionData.InvokedFunctionArn,
		}
	}

	var jsonWriter fastjson.Writer
	if err := lc.MarshalFastJSON(&jsonWriter); err != nil {
		return apmproxy.AgentData{}, err
	}

	var logData []byte
	if metadataContainer.Metadata != nil {
		logData = append(metadataContainer.Metadata, []byte("\n")...)
	}

	logData = append(logData, jsonWriter.Bytes()...)
	return apmproxy.AgentData{Data: logData}, nil
}
