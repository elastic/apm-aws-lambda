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
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"

	"elastic/apm-lambda-extension/model"

	"github.com/pkg/errors"
)

type MetadataContainer struct {
	Metadata *model.Metadata `json:"metadata"`
}

func ProcessMetadata(data AgentData, container *MetadataContainer) error {
	uncompressedData, err := GetUncompressedBytes(data.Data, data.ContentEncoding)
	if err != nil {
		return errors.New(fmt.Sprintf("Error uncompressing agent data for metadata extraction : %v", err))
	}
	decoder := json.NewDecoder(bytes.NewReader(uncompressedData))
	for {
		err = decoder.Decode(container)
		if container.Metadata != nil {
			Log.Debug("Metadata decoded")
			break
		}
		if err != nil {
			if err == io.EOF {
				return errors.New("No metadata in current agent transaction")
			} else {
				return errors.New(fmt.Sprintf("Error uncompressing agent data for metadata extraction : %v", err))
			}
		}
	}
	return nil
}

func GetUncompressedBytes(rawBytes []byte, encodingType string) ([]byte, error) {
	switch encodingType {
	case "deflate":
		reader := bytes.NewReader([]byte(rawBytes))
		zlibreader, err := zlib.NewReader(reader)
		if err != nil {
			return nil, fmt.Errorf("could not create zlib.NewReader: %v", err)
		}
		bodyBytes, err := ioutil.ReadAll(zlibreader)
		if err != nil {
			return nil, fmt.Errorf("could not read from zlib reader using ioutil.ReadAll: %v", err)
		}
		return bodyBytes, nil
	case "gzip":
		reader := bytes.NewReader([]byte(rawBytes))
		zlibreader, err := gzip.NewReader(reader)
		if err != nil {
			return nil, fmt.Errorf("could not create gzip.NewReader: %v", err)
		}
		bodyBytes, err := ioutil.ReadAll(zlibreader)
		if err != nil {
			return nil, fmt.Errorf("could not read from gzip reader using ioutil.ReadAll: %v", err)
		}
		return bodyBytes, nil
	default:
		return rawBytes, nil
	}
}
