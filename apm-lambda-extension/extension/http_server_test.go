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
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func Test_getDecompressedBytesFromRequestUncompressed(t *testing.T) {
	s := "A long time ago in a galaxy far, far away..."
	body := strings.NewReader(s)

	// Create the request
	req, err := http.NewRequest(http.MethodPost, "example.com", body)
	if err != nil {
		t.Errorf("Error creating new request: %v", err)
		t.Fail()
	}

	// Decompress the request's body
	got, err1 := getDecompressedBytesFromRequest(req)
	if err1 != nil {
		t.Errorf("Error decompressing request body: %v", err1)
		t.Fail()
	}

	if s != string(got) {
		t.Errorf("Original string and decompressed data do not match")
		t.Fail()
	}
}

func Test_getDecompressedBytesFromRequestGzip(t *testing.T) {
	s := "A long time ago in a galaxy far, far away..."
	var b bytes.Buffer

	// Compress the data
	w := gzip.NewWriter(&b)
	w.Write([]byte(s))
	w.Close()

	// Create a reader reading from the bytes on the buffer
	body := bytes.NewReader(b.Bytes())

	// Create the request
	req, err := http.NewRequest(http.MethodPost, "example.com", body)
	if err != nil {
		t.Errorf("Error creating new request: %v", err)
		t.Fail()
	}

	// Set the encoding to gzip
	req.Header.Set("Content-Encoding", "gzip")

	// Decompress the request's body
	got, err1 := getDecompressedBytesFromRequest(req)
	if err1 != nil {
		t.Errorf("Error decompressing request body: %v", err1)
		t.Fail()
	}

	if s != string(got) {
		t.Errorf("Original string and decompressed data do not match")
		t.Fail()
	}
}

func Test_getDecompressedBytesFromRequestDeflate(t *testing.T) {
	s := "A long time ago in a galaxy far, far away..."
	var b bytes.Buffer

	// Compress the data
	w := zlib.NewWriter(&b)
	w.Write([]byte(s))
	w.Close()

	// Create a reader reading from the bytes on the buffer
	body := bytes.NewReader(b.Bytes())

	// Create the request
	req, err := http.NewRequest(http.MethodPost, "example.com", body)
	if err != nil {
		t.Errorf("Error creating new request: %v", err)
		t.Fail()
	}

	// Set the encoding to deflate
	req.Header.Set("Content-Encoding", "deflate")

	// Decompress the request's body
	got, err1 := getDecompressedBytesFromRequest(req)
	if err1 != nil {
		t.Errorf("Error decompressing request body: %v", err1)
		t.Fail()
	}

	if s != string(got) {
		t.Errorf("Original string and decompressed data do not match")
		t.Fail()
	}
}

func Test_getDecompressedBytesFromRequestEmptyBody(t *testing.T) {
	// Create the request
	req, err := http.NewRequest(http.MethodPost, "example.com", nil)
	if err != nil {
		t.Errorf("Error creating new request: %v", err)
	}

	got, err := getDecompressedBytesFromRequest(req)
	if err != nil {
		t.Errorf("Error decompressing request body: %v", err)
	}

	if len(got) != 0 {
		t.Errorf("A non-empty byte slice was returned")
		t.Fail()
	}
}

/**
 * Starts a mock HTTP server
 *
 * Exposes a single / endpoint.  This endpoint returns
 * a JSON string with two properties
 *
 * data: the value passed via `response`, as JSON
 * seen_headers: the headers seen by the request to /
 *
 * The intended use is to start the extension's HTTP server
 * and confirm that it proxies info requests to this server
 */
func startMockApmServer(response map[string]string, port string, wg *sync.WaitGroup) {
	handler := &mockServerHandler{response: response}
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("could not setup mock apm server listener: %v", err)
	}
	// http.Serve blocks, so we can't defer the wg.Done call
	// but the tcp listener should be accepting connections
	// at this point, so we can call wg.Done here
	wg.Done()
	if err := http.Serve(listener, handler); err != nil {
		log.Fatalf("could not server mock apm server: %v", err)
	}
}

type mockServerHandler struct {
	response map[string]string
}

func (handler *mockServerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	seenHeaders := make(map[string]string)
	for name, values := range r.Header {
		for _, value := range values {
			seenHeaders[name] = value
		}
	}

	data := map[string]map[string]string{
		"data":         handler.response,
		"seen_headers": seenHeaders,
	}
	responseString, _ := json.Marshal(data)
	w.Write([]byte(responseString))
}

func getUrl(url string, headers map[string]string, t *testing.T) string {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	for name, value := range headers {
		req.Header.Add(name, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Logf("Error fetching %s, [%v]", url, err)
		return "{}"
	}

	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	return string(body)
}

func startExtension(config *extensionConfig, wg *sync.WaitGroup) {
	// NewHttpServer does not block, so we can defer
	// the wg.Done call
	defer wg.Done()
	dataChannel := make(chan []byte, 100)
	NewHttpServer(dataChannel, config)
}

func getRandomNetworkPort(notwanted []int) int {
	rand.Seed(time.Now().UnixNano())
	min := 50000
	max := 65534
	var value = rand.Intn(max-min+1) + min
	// if we've got a value that's not wanted,
	// recall outself recursivly
	for _, v := range notwanted {
		if value == v {
			return getRandomNetworkPort(notwanted)
		}
	}
	return value
}

func TestProxy(t *testing.T) {
	var iApmServerPort = getRandomNetworkPort([]int{})
	apmServerPort := fmt.Sprint(iApmServerPort)
	var extensionPort = fmt.Sprint(getRandomNetworkPort([]int{iApmServerPort}))

	var wg sync.WaitGroup
	wg.Add(1)

	go startExtension(&extensionConfig{
		apmServerUrl:               "http://localhost:" + apmServerPort,
		apmServerSecretToken:       "foo",
		apmServerApiKey:            "bar",
		dataReceiverServerPort:     ":" + extensionPort,
		dataReceiverTimeoutSeconds: 15,
	}, &wg)

	wg.Add(1)
	go startMockApmServer(
		map[string]string{"foo": "bar"},
		apmServerPort,
		&wg,
	)

	wg.Wait()

	body := getUrl(
		"http://localhost:"+extensionPort,
		map[string]string{"Authorization": "test-value"},
		t,
	)

	var result map[string]map[string]string
	json.Unmarshal([]byte(body), &result)
	var data map[string]string

	if result["data"]["foo"] != "bar" {
		t.Logf("unexpected value returned: %s", data)
		t.Fail()
	}

	if result["seen_headers"]["Authorization"] != "test-value" {
		t.Logf("did not see Authorization header %s", data)
		t.Fail()
	}
}
