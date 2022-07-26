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
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
)

func TestProcessEnv(t *testing.T) {
	sm := new(mockSecretManager)
	t.Setenv("ELASTIC_APM_LAMBDA_APM_SERVER", "bar.example.com/")
	t.Setenv("ELASTIC_APM_SECRET_TOKEN", "foo")
	config := ProcessEnv(sm)
	t.Logf("%v", config)

	if config.apmServerUrl != "bar.example.com/" {
		t.Logf("Endpoint not set correctly: %s", config.apmServerUrl)
		t.Fail()
	}

	t.Setenv("ELASTIC_APM_LAMBDA_APM_SERVER", "foo.example.com")
	t.Setenv("ELASTIC_APM_SECRET_TOKEN", "bar")

	config = ProcessEnv(sm)
	t.Logf("%v", config)

	// config normalizes string to ensure it ends in a `/`
	if config.apmServerUrl != "foo.example.com/" {
		t.Logf("Endpoint not set correctly: %s", config.apmServerUrl)
		t.Fail()
	}

	if config.ApmServerSecretToken != "bar" {
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

	t.Setenv("ELASTIC_APM_DATA_RECEIVER_SERVER_PORT", "8201")
	config = ProcessEnv(sm)
	if config.dataReceiverServerPort != ":8201" {
		t.Log("Env port not set correctly")
		t.Fail()
	}

	t.Setenv("ELASTIC_APM_DATA_RECEIVER_TIMEOUT_SECONDS", "10")
	config = ProcessEnv(sm)
	if config.dataReceiverTimeoutSeconds != 10 {
		t.Log("APM data receiver timeout not set correctly")
		t.Fail()
	}

	t.Setenv("ELASTIC_APM_DATA_RECEIVER_TIMEOUT_SECONDS", "foo")
	config = ProcessEnv(sm)
	if config.dataReceiverTimeoutSeconds != 15 {
		t.Log("APM data receiver timeout not set correctly")
		t.Fail()
	}

	t.Setenv("ELASTIC_APM_DATA_FORWARDER_TIMEOUT_SECONDS", "10")
	config = ProcessEnv(sm)
	if config.DataForwarderTimeoutSeconds != 10 {
		t.Log("APM data forwarder timeout not set correctly")
		t.Fail()
	}

	t.Setenv("ELASTIC_APM_DATA_FORWARDER_TIMEOUT_SECONDS", "foo")
	config = ProcessEnv(sm)
	if config.DataForwarderTimeoutSeconds != 3 {
		t.Log("APM data forwarder not set correctly")
		t.Fail()
	}

	t.Setenv("ELASTIC_APM_API_KEY", "foo")
	config = ProcessEnv(sm)
	if config.ApmServerApiKey != "foo" {
		t.Log("API Key not set correctly")
		t.Fail()
	}

	t.Setenv("ELASTIC_APM_SEND_STRATEGY", "Background")
	config = ProcessEnv(sm)
	if config.SendStrategy != "background" {
		t.Log("Background send strategy not set correctly")
		t.Fail()
	}

	t.Setenv("ELASTIC_APM_SEND_STRATEGY", "invalid")
	config = ProcessEnv(sm)
	if config.SendStrategy != "syncflush" {
		t.Log("Syncflush send strategy not set correctly")
		t.Fail()
	}

	t.Setenv("ELASTIC_APM_LOG_LEVEL", "debug")
	config = ProcessEnv(sm)
	if config.LogLevel != zapcore.DebugLevel {
		t.Log("Log level not set correctly")
		t.Fail()
	}

	t.Setenv("ELASTIC_APM_LOG_LEVEL", "invalid")
	config = ProcessEnv(sm)
	if config.LogLevel != zapcore.InfoLevel {
		t.Log("Log level not set correctly")
		t.Fail()
	}
}

func TestGetSecretCalled(t *testing.T) {
	t.Setenv("ELASTIC_APM_LAMBDA_APM_SERVER", "bar.example.com/")
	t.Setenv("ELASTIC_APM_SECRET_TOKEN", "unmanagedsecret")
	t.Setenv("ELASTIC_APM_API_KEY", "unmanagedapikey")
	t.Setenv("ELASTIC_APM_SECRETS_MANAGER_SECRET_TOKEN_ID", "secrettoken")
	t.Setenv("ELASTIC_APM_SECRETS_MANAGER_API_KEY_ID", "apikey")

	sm := new(mockSecretManager)

	config := ProcessEnv(sm)
	assert.Equal(t, "secrettoken", config.ApmServerSecretToken)
	assert.Equal(t, "apikey", config.ApmServerApiKey)

	t.Setenv("ELASTIC_APM_SECRETS_MANAGER_SECRET_TOKEN_ID", "")
	t.Setenv("ELASTIC_APM_SECRETS_MANAGER_API_KEY_ID", "")

	config = ProcessEnv(sm)
	assert.Equal(t, "unmanagedsecret", config.ApmServerSecretToken)
	assert.Equal(t, "unmanagedapikey", config.ApmServerApiKey)
}

type mockSecretManager struct{}

func (s *mockSecretManager) GetSecretValue(ctx context.Context, input *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	switch s := *input.SecretId; s {
	case "secrettoken":
		return &secretsmanager.GetSecretValueOutput{SecretString: input.SecretId}, nil
	case "apikey":
		data := []byte(s)
		secretBinary := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
		base64.StdEncoding.Encode(secretBinary, data)
		return &secretsmanager.GetSecretValueOutput{SecretBinary: secretBinary}, nil
	default:
		return nil, fmt.Errorf("unrecognized secret input value %s", s)
	}
}
