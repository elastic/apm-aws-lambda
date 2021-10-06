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
	"net/http"
	"strings"
	"testing"
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
