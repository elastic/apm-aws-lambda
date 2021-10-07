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

package extension

import (
	"encoding/json"
	"log"
)

func ProcessShutdown() {
	log.Println("Received SHUTDOWN event")
	log.Println("Exiting")
}

func FlushAPMData(dataChannel chan []byte, config *extensionConfig) {
	log.Println("Checking for agent data")
	for {
		select {
		case agentData := <-dataChannel:
			log.Println("Processing agent data")
			err := PostToApmServer(agentData, config)
			if err != nil {
				log.Printf("Error sending to APM server, skipping: %v", err)
			}
		default:
			log.Println("No agent data on buffer")
			return
		}
	}
}

func PrettyPrint(v interface{}) string {
	data, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return ""
	}
	return string(data)
}
