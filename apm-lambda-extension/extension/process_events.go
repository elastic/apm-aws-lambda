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

package extension

import (
	"context"
	"encoding/json"
)

// FlushAPMData reads all the apm data in the apm data channel and sends it to the APM server.
func FlushAPMData(ctx context.Context, transport *ApmServerTransport) {
	if transport.Status == Failing {
		Log.Debug("Flush skipped - Transport failing")
		return
	}
	Log.Debug("Flush started - Checking for agent data")
	for {
		select {
		case agentData := <-transport.DataChannel:
			Log.Debug("Flush in progress - Processing agent data")
			if err := PostToApmServer(ctx, transport, agentData); err != nil {
				Log.Errorf("Error sending to APM server, skipping: %v", err)
			}
		default:
			Log.Debug("Flush ended - No agent data on buffer")
			return
		}
	}
}

// PrettyPrint prints formatted, legible json data.
func PrettyPrint(v interface{}) string {
	data, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return ""
	}
	return string(data)
}
