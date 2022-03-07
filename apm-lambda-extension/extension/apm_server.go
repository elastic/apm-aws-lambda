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
	"io/ioutil"
	"log"
	"net/http"
	"sync"
)

var bufferPool = sync.Pool{New: func() interface{} {
	return &bytes.Buffer{}
}}

// todo: can this be a streaming or streaming style call that keeps the
//       connection open across invocations?
func PostToApmServer(client *http.Client, agentData AgentData, config *extensionConfig) error {
	endpointURI := "intake/v2/events"
	encoding := agentData.ContentEncoding
	buf := bufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufferPool.Put(buf)
	}()

	if agentData.ContentEncoding == "" {
		encoding = "gzip"
		gw, err := gzip.NewWriterLevel(buf, gzip.BestSpeed)
		if err != nil {
			return err
		}
		if _, err := gw.Write(agentData.Data); err != nil {
			log.Printf("Failed to compress data: %v", err)
		}
		if err := gw.Close(); err != nil {
			log.Printf("Failed write compressed data to buffer: %v", err)
		}
	} else {
		buf.Write(agentData.Data)
	}

	req, err := http.NewRequest("POST", config.apmServerUrl+endpointURI, buf)
	req.Close = true
	if err != nil {
		return fmt.Errorf("failed to create a new request when posting to APM server: %v", err)
	}
	req.Header.Add("Content-Encoding", encoding)
	req.Header.Add("Content-Type", "application/x-ndjson")
	if config.apmServerApiKey != "" {
		req.Header.Add("Authorization", "ApiKey "+config.apmServerApiKey)
	} else if config.apmServerSecretToken != "" {
		req.Header.Add("Authorization", "Bearer "+config.apmServerSecretToken)
	}

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
