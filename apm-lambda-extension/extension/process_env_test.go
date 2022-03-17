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
	"github.com/sirupsen/logrus"
	"os"
	"testing"
)

func TestProcessEnv(t *testing.T) {
	os.Setenv("ELASTIC_APM_LAMBDA_APM_SERVER", "bar.example.com/")
	os.Setenv("ELASTIC_APM_SECRET_TOKEN", "foo")
	config := ProcessEnv()
	t.Logf("%v", config)

	if config.apmServerUrl != "bar.example.com/" {
		t.Logf("Endpoint not set correctly: %s", config.apmServerUrl)
		t.Fail()
	}

	os.Setenv("ELASTIC_APM_LAMBDA_APM_SERVER", "foo.example.com")
	os.Setenv("ELASTIC_APM_SECRET_TOKEN", "bar")

	config = ProcessEnv()
	t.Logf("%v", config)

	// config normalizes string to ensure it ends in a `/`
	if config.apmServerUrl != "foo.example.com/" {
		t.Logf("Endpoint not set correctly: %s", config.apmServerUrl)
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

	if config.SendStrategy != SyncFlush {
		t.Log("Default send strategy not set correctly")
		t.Fail()
	}

	os.Setenv("ELASTIC_APM_DATA_RECEIVER_SERVER_PORT", "8201")
	config = ProcessEnv()
	if config.dataReceiverServerPort != ":8201" {
		t.Log("Env port not set correctly")
		t.Fail()
	}

	os.Setenv("ELASTIC_APM_DATA_RECEIVER_TIMEOUT_SECONDS", "10")
	config = ProcessEnv()
	if config.dataReceiverTimeoutSeconds != 10 {
		t.Log("APM data receiver timeout not set correctly")
		t.Fail()
	}

	os.Setenv("ELASTIC_APM_DATA_RECEIVER_TIMEOUT_SECONDS", "foo")
	config = ProcessEnv()
	if config.dataReceiverTimeoutSeconds != 15 {
		t.Log("APM data receiver timeout not set correctly")
		t.Fail()
	}

	os.Setenv("ELASTIC_APM_DATA_FORWARDER_TIMEOUT_SECONDS", "10")
	config = ProcessEnv()
	if config.DataForwarderTimeoutSeconds != 10 {
		t.Log("APM data forwarder timeout not set correctly")
		t.Fail()
	}

	os.Setenv("ELASTIC_APM_DATA_FORWARDER_TIMEOUT_SECONDS", "foo")
	config = ProcessEnv()
	if config.DataForwarderTimeoutSeconds != 3 {
		t.Log("APM data forwarder not set correctly")
		t.Fail()
	}

	os.Setenv("ELASTIC_APM_API_KEY", "foo")
	config = ProcessEnv()
	if config.apmServerApiKey != "foo" {
		t.Log("API Key not set correctly")
		t.Fail()
	}

	os.Setenv("ELASTIC_APM_SEND_STRATEGY", "Background")
	config = ProcessEnv()
	if config.SendStrategy != "background" {
		t.Log("Background send strategy not set correctly")
		t.Fail()
	}

	os.Setenv("ELASTIC_APM_SEND_STRATEGY", "invalid")
	config = ProcessEnv()
	if config.SendStrategy != "syncflush" {
		t.Log("Syncflush send strategy not set correctly")
		t.Fail()
	}

	os.Setenv("ELASTIC_APM_LOG_LEVEL", "debug")
	config = ProcessEnv()
	if config.LogLevel != logrus.DebugLevel {
		t.Log("Log level not set correctly")
		t.Fail()
	}

	os.Setenv("ELASTIC_APM_LOG_LEVEL", "invalid")
	config = ProcessEnv()
	if config.LogLevel != logrus.InfoLevel {
		t.Log("Log level not set correctly")
		t.Fail()
	}
}
