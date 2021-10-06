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

package logsapi

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
)

const DefaultHttpListenerPort = "1234"

// Init initializes the configuration for the Logs API and subscribes to the Logs API for HTTP
func Subscribe(extensionID string, eventTypes []EventType) error {
	extensions_api_address, ok := os.LookupEnv("AWS_LAMBDA_RUNTIME_API")
	if !ok {
		return errors.New("AWS_LAMBDA_RUNTIME_API is not set")
	}

	logsAPIBaseUrl := fmt.Sprintf("http://%s", extensions_api_address)

	logsAPIClient, err := NewClient(logsAPIBaseUrl)
	if err != nil {
		return err
	}

	bufferingCfg := BufferingCfg{
		MaxItems:  10000,
		MaxBytes:  262144,
		TimeoutMS: 1000,
	}
	if err != nil {
		return err
	}
	address := ListenOnAddress()
	destination := Destination{
		Protocol:   HttpProto,
		URI:        URI("http://" + address),
		HttpMethod: HttpPost,
		Encoding:   JSON,
	}

	_, err = logsAPIClient.Subscribe(eventTypes, bufferingCfg, destination, extensionID)
	return err
}
