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
	"elastic/apm-lambda-extension/logger"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type extensionConfig struct {
	apmServerUrl                string
	ApmServerSecretToken        string
	ApmServerApiKey             string
	dataReceiverServerPort      string
	SendStrategy                SendStrategy
	dataReceiverTimeoutSeconds  int
	DataForwarderTimeoutSeconds int
	LogLevel                    zapcore.Level
}

// SendStrategy represents the type of sending strategy the extension uses
type SendStrategy string

const (
	// Background send strategy allows the extension to send remaining buffered
	// agent data on the next function invocation
	Background SendStrategy = "background"

	// SyncFlush send strategy indicates that the extension will synchronously
	// flush remaining buffered agent data when it receives a signal that the
	// function is complete
	SyncFlush SendStrategy = "syncflush"

	defaultDataReceiverTimeoutSeconds  int = 15
	defaultDataForwarderTimeoutSeconds int = 3
)

func getIntFromEnv(name string) (int, error) {
	strValue := os.Getenv(name)
	value, err := strconv.Atoi(strValue)
	if err != nil {
		return -1, err
	}
	return value, nil
}

type secretManager interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

func getSecret(manager secretManager, secretName string) (string, error) {
	input := &secretsmanager.GetSecretValueInput{
		SecretId:     ptrFromString(secretName),
		VersionStage: ptrFromString("AWSCURRENT"),
	}

	result, err := manager.GetSecretValue(context.TODO(), input)
	if err != nil {
		return "", err
	}

	var secretString string
	if result.SecretString != nil {
		secretString = *result.SecretString
	} else {
		decodedBinarySecretBytes := make([]byte, base64.StdEncoding.DecodedLen(len(result.SecretBinary)))
		_, err := base64.StdEncoding.Decode(decodedBinarySecretBytes, result.SecretBinary)
		if err != nil {
			return "", err
		}
		secretString = string(decodedBinarySecretBytes)
	}

	return secretString, nil
}

func ptrFromString(v string) *string {
	return &v
}

// ProcessEnv extracts ENV variables into globals
func ProcessEnv(manager secretManager, log *zap.SugaredLogger) *extensionConfig {
	dataReceiverTimeoutSeconds, err := getIntFromEnv("ELASTIC_APM_DATA_RECEIVER_TIMEOUT_SECONDS")
	if err != nil {
		dataReceiverTimeoutSeconds = defaultDataReceiverTimeoutSeconds
		log.Warnf("Could not read ELASTIC_APM_DATA_RECEIVER_TIMEOUT_SECONDS, defaulting to %d: %v", dataReceiverTimeoutSeconds, err)
	}

	dataForwarderTimeoutSeconds, err := getIntFromEnv("ELASTIC_APM_DATA_FORWARDER_TIMEOUT_SECONDS")
	if err != nil {
		dataForwarderTimeoutSeconds = defaultDataForwarderTimeoutSeconds
		log.Warnf("Could not read ELASTIC_APM_DATA_FORWARDER_TIMEOUT_SECONDS, defaulting to %d: %v", dataForwarderTimeoutSeconds, err)
	}

	// add trailing slash to server name if missing
	normalizedApmLambdaServer := os.Getenv("ELASTIC_APM_LAMBDA_APM_SERVER")
	if normalizedApmLambdaServer != "" && normalizedApmLambdaServer[len(normalizedApmLambdaServer)-1:] != "/" {
		normalizedApmLambdaServer = normalizedApmLambdaServer + "/"
	}

	logLevel, err := logger.ParseLogLevel(strings.ToLower(os.Getenv("ELASTIC_APM_LOG_LEVEL")))
	if err != nil {
		logLevel = zapcore.InfoLevel
		log.Warnf("Could not read ELASTIC_APM_LOG_LEVEL, defaulting to %s", logLevel)
	}

	// Get the send strategy, convert to lowercase
	normalizedSendStrategy := SyncFlush
	sendStrategy := strings.ToLower(os.Getenv("ELASTIC_APM_SEND_STRATEGY"))
	if sendStrategy == string(Background) {
		normalizedSendStrategy = Background
	}

	apmServerApiKey := os.Getenv("ELASTIC_APM_API_KEY")
	apmServerApiKeySMSecretId := os.Getenv("ELASTIC_APM_SECRETS_MANAGER_API_KEY_ID")
	if apmServerApiKeySMSecretId != "" {
		result, err := getSecret(manager, apmServerApiKeySMSecretId)
		if err != nil {
			log.Fatalf("Failed loading APM Server ApiKey from Secrets Manager: %v", err)
		}
		log.Infof("Using the APM API key retrieved from Secrets Manager.")
		apmServerApiKey = result
	}

	apmServerSecretToken := os.Getenv("ELASTIC_APM_SECRET_TOKEN")
	apmServerSecretTokenSMSecretId := os.Getenv("ELASTIC_APM_SECRETS_MANAGER_SECRET_TOKEN_ID")
	if apmServerSecretTokenSMSecretId != "" {
		result, err := getSecret(manager, apmServerSecretTokenSMSecretId)
		if err != nil {
			log.Fatalf("Failed loading APM Server Secret Token from Secrets Manager: %v", err)
		}
		log.Infof("Using the APM secret token retrieved from Secrets Manager.")
		apmServerSecretToken = result
	}

	config := &extensionConfig{
		apmServerUrl:                normalizedApmLambdaServer,
		ApmServerSecretToken:        apmServerSecretToken,
		ApmServerApiKey:             apmServerApiKey,
		dataReceiverServerPort:      fmt.Sprintf(":%s", os.Getenv("ELASTIC_APM_DATA_RECEIVER_SERVER_PORT")),
		SendStrategy:                normalizedSendStrategy,
		dataReceiverTimeoutSeconds:  dataReceiverTimeoutSeconds,
		DataForwarderTimeoutSeconds: dataForwarderTimeoutSeconds,
		LogLevel:                    logLevel,
	}

	if config.dataReceiverServerPort == ":" {
		config.dataReceiverServerPort = ":8200"
	}
	if config.apmServerUrl == "" {
		log.Fatal("please set ELASTIC_APM_LAMBDA_APM_SERVER, exiting")
	}
	if config.ApmServerSecretToken == "" && config.ApmServerApiKey == "" {
		log.Warn("ELASTIC_APM_SECRET_TOKEN or ELASTIC_APM_API_KEY not specified")
	}

	return config
}
