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
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
)

// todo: can this be a streaming or streaming style call that keeps the
//       connection open across invocations?
func PostToApmServer(agentData AgentData, config *extensionConfig) error {
	endpointUri := "intake/v2/events"
	var req *http.Request
	var err error

	if agentData.ContentEncoding == "" {
		pr, pw := io.Pipe()
		gw, _ := gzip.NewWriterLevel(pw, gzip.BestSpeed)
		var err error

		go func() {
			_, err = io.Copy(gw, bytes.NewReader(agentData.Data))
			gw.Close()
			pw.Close()
		}()

		// Todo: Handle the err (failure to compress the data)

		req, err = http.NewRequest("POST", config.apmServerUrl+endpointUri, pr)
		if err != nil {
			return fmt.Errorf("failed to create a new request when posting to APM server: %v", err)
		}
		req.Header.Add("Content-Encoding", "gzip")
	} else {
		req, err = http.NewRequest("POST", config.apmServerUrl+endpointUri, bytes.NewReader(agentData.Data))
		if err != nil {
			return fmt.Errorf("failed to create a new request when posting to APM server: %v", err)
		}
		req.Header.Add("Content-Encoding", agentData.ContentEncoding)
	}

	req.Header.Add("Content-Type", "application/x-ndjson")
	if config.apmServerApiKey != "" {
		req.Header.Add("Authorization", "ApiKey "+config.apmServerApiKey)
	} else if config.apmServerSecretToken != "" {
		req.Header.Add("Authorization", "Bearer "+config.apmServerSecretToken)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to post to APM server: %v", err)
	}

	//Read the response body
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read the response body after posting to the APM server")
	}

	log.Printf("APM server response body: %v\n", string(body))
	log.Printf("APM server response status code: %v\n", resp.StatusCode)
	return nil
}
