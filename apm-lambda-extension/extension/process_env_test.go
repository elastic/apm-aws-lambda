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
	"os"
	"testing"
)

func TestProcessEnv(t *testing.T) {
	os.Setenv("ELASTIC_APM_LAMBDA_APM_SERVER", "foo.example.com")
	os.Setenv("ELASTIC_APM_SECRET_TOKEN", "bar")

	config := ProcessEnv()
	t.Logf("%v", config)

	if config.apmServerEndpoint != "foo.example.com/intake/v2/events" {
		t.Log("Endpoint not set correctly")
		t.Fail()
	}

	if config.apmServerSecretToken != "bar" {
		t.Log("Secret Token not set correctly")
		t.Fail()
	}

	if config.dataReceiverServerPort != ":8200" {
		t.Log("Default port not set correctly")
		t.Fail()
	}

	if config.dataReceiverTimeoutSeconds != 15 {
		t.Log("Default timeout not set correctly")
		t.Fail()
	}

	os.Setenv("ELASTIC_APM_DATA_RECEIVER_SERVER_PORT", ":8201")
	config = ProcessEnv()
	if config.dataReceiverServerPort != ":8201" {
		t.Log("Env port not set correctly")
		t.Fail()
	}

	os.Setenv("ELASTIC_APM_DATA_RECEIVER_TIMEOUT_SECONDS", "10")
	config = ProcessEnv()
	if config.dataReceiverTimeoutSeconds != 10 {
		t.Log("Timeout not set correctly")
		t.Fail()
	}

	os.Setenv("ELASTIC_APM_DATA_RECEIVER_TIMEOUT_SECONDS", "foo")
	config = ProcessEnv()
	if config.dataReceiverTimeoutSeconds != 15 {
		t.Log("Timeout not set correctly")
		t.Fail()
	}

	os.Setenv("ELASTIC_APM_API_KEY", "foo")
	config = ProcessEnv()
	if config.apmServerApiKey != "foo" {
		t.Log("API Key not set correctly")
		t.Fail()
	}
}
