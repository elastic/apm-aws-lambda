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
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

func TestProcessEnv(t *testing.T) {
	sm := new(mockSecretManager)
	if err := os.Setenv("ELASTIC_APM_LAMBDA_APM_SERVER", "bar.example.com/"); err != nil {
		t.Fail()
		return
	}
	if err := os.Setenv("ELASTIC_APM_SECRET_TOKEN", "foo"); err != nil {
		t.Fail()
		return
	}
	config := ProcessEnv(sm)
	t.Logf("%v", config)

	if config.apmServerUrl != "bar.example.com/" {
		t.Logf("Endpoint not set correctly: %s", config.apmServerUrl)
		t.Fail()
	}

	if err := os.Setenv("ELASTIC_APM_LAMBDA_APM_SERVER", "foo.example.com"); err != nil {
		t.Fail()
		return
	}
	if err := os.Setenv("ELASTIC_APM_SECRET_TOKEN", "bar"); err != nil {
		t.Fail()
		return
	}

	config = ProcessEnv(sm)
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

	if err := os.Setenv("ELASTIC_APM_DATA_RECEIVER_SERVER_PORT", "8201"); err != nil {
		t.Fail()
		return
	}
	config = ProcessEnv(sm)
	if config.dataReceiverServerPort != ":8201" {
		t.Log("Env port not set correctly")
		t.Fail()
	}

	if err := os.Setenv("ELASTIC_APM_DATA_RECEIVER_TIMEOUT_SECONDS", "10"); err != nil {
		t.Fail()
		return
	}
	config = ProcessEnv(sm)
	if config.dataReceiverTimeoutSeconds != 10 {
		t.Log("APM data receiver timeout not set correctly")
		t.Fail()
	}

	if err := os.Setenv("ELASTIC_APM_DATA_RECEIVER_TIMEOUT_SECONDS", "foo"); err != nil {
		t.Fail()
		return
	}
	config = ProcessEnv(sm)
	if config.dataReceiverTimeoutSeconds != 15 {
		t.Log("APM data receiver timeout not set correctly")
		t.Fail()
	}

	if err := os.Setenv("ELASTIC_APM_DATA_FORWARDER_TIMEOUT_SECONDS", "10"); err != nil {
		t.Fail()
		return
	}
	config = ProcessEnv(sm)
	if config.DataForwarderTimeoutSeconds != 10 {
		t.Log("APM data forwarder timeout not set correctly")
		t.Fail()
	}

	if err := os.Setenv("ELASTIC_APM_DATA_FORWARDER_TIMEOUT_SECONDS", "foo"); err != nil {
		t.Fail()
		return
	}
	config = ProcessEnv(sm)
	if config.DataForwarderTimeoutSeconds != 3 {
		t.Log("APM data forwarder not set correctly")
		t.Fail()
	}

	if err := os.Setenv("ELASTIC_APM_API_KEY", "foo"); err != nil {
		t.Fail()
		return
	}
	config = ProcessEnv(sm)
	if config.apmServerApiKey != "foo" {
		t.Log("API Key not set correctly")
		t.Fail()
	}

	if err := os.Setenv("ELASTIC_APM_SEND_STRATEGY", "Background"); err != nil {
		t.Fail()
		return
	}
	config = ProcessEnv(sm)
	if config.SendStrategy != "background" {
		t.Log("Background send strategy not set correctly")
		t.Fail()
	}

	if err := os.Setenv("ELASTIC_APM_SEND_STRATEGY", "invalid"); err != nil {
		t.Fail()
		return
	}
	config = ProcessEnv(sm)
	if config.SendStrategy != "syncflush" {
		t.Log("Syncflush send strategy not set correctly")
		t.Fail()
	}

	if err := os.Setenv("ELASTIC_APM_LOG_LEVEL", "debug"); err != nil {
		t.Fail()
		return
	}
	config = ProcessEnv(sm)
	if config.LogLevel != zapcore.DebugLevel {
		t.Log("Log level not set correctly")
		t.Fail()
	}

	if err := os.Setenv("ELASTIC_APM_LOG_LEVEL", "invalid"); err != nil {
		t.Fail()
		return
	}
	config = ProcessEnv(sm)
	if config.LogLevel != zapcore.InfoLevel {
		t.Log("Log level not set correctly")
		t.Fail()
	}
}

func TestGetSecretCalled(t *testing.T) {
	originalSecretToken := os.Getenv("ELASTIC_APM_SECRETS_MANAGER_SECRET_TOKEN_ID")
	originalApiKey := os.Getenv("ELASTIC_APM_SECRETS_MANAGER_API_KEY_ID")
	originalUnmanagedSecretToken := os.Getenv("ELASTIC_APM_SECRET_TOKEN")
	originalUnmanagedApiKey := os.Getenv("ELASTIC_APM_API_KEY")
	defer func() {
		os.Setenv("ELASTIC_APM_SECRETS_MANAGER_SECRET_TOKEN_ID", originalSecretToken)
		os.Setenv("ELASTIC_APM_SECRETS_MANAGER_API_KEY_ID", originalApiKey)
		os.Setenv("ELASTIC_APM_SECRET_TOKEN", originalUnmanagedSecretToken)
		os.Setenv("ELASTIC_APM_API_KEY", originalUnmanagedApiKey)
	}()

	require.NoError(t, os.Setenv("ELASTIC_APM_LAMBDA_APM_SERVER", "bar.example.com/"))
	require.NoError(t, os.Setenv("ELASTIC_APM_SECRET_TOKEN", "unmanagedsecret"))
	require.NoError(t, os.Setenv("ELASTIC_APM_API_KEY", "unmanagedapikey"))
	require.NoError(t, os.Setenv("ELASTIC_APM_SECRETS_MANAGER_SECRET_TOKEN_ID", "secrettoken"))
	require.NoError(t, os.Setenv("ELASTIC_APM_SECRETS_MANAGER_API_KEY_ID", "apikey"))

	sm := new(mockSecretManager)

	config := ProcessEnv(sm)
	assert.Equal(t, "secrettoken", config.apmServerSecretToken)
	assert.Equal(t, "apikey", config.apmServerApiKey)

	require.NoError(t, os.Setenv("ELASTIC_APM_SECRETS_MANAGER_SECRET_TOKEN_ID", ""))
	require.NoError(t, os.Setenv("ELASTIC_APM_SECRETS_MANAGER_API_KEY_ID", ""))

	config = ProcessEnv(sm)
	assert.Equal(t, "unmanagedsecret", config.apmServerSecretToken)
	assert.Equal(t, "unmanagedapikey", config.apmServerApiKey)
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
