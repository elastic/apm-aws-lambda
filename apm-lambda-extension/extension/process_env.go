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
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

type extensionConfig struct {
	apmServerUrl                string
	apmServerSecretToken        string
	apmServerApiKey             string
	dataReceiverServerPort      string
	SendStrategy                SendStrategy
	dataReceiverTimeoutSeconds  int
	DataForwarderTimeoutSeconds int
	LogLevel                    logrus.Level
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

// ProcessEnv extracts ENV variables into globals
func ProcessEnv() *extensionConfig {
	dataReceiverTimeoutSeconds, err := getIntFromEnv("ELASTIC_APM_DATA_RECEIVER_TIMEOUT_SECONDS")
	if err != nil {
		dataReceiverTimeoutSeconds = defaultDataReceiverTimeoutSeconds
		Log.Warnf("Could not read ELASTIC_APM_DATA_RECEIVER_TIMEOUT_SECONDS, defaulting to %d: %v", dataReceiverTimeoutSeconds, err)
	}

	dataForwarderTimeoutSeconds, err := getIntFromEnv("ELASTIC_APM_DATA_FORWARDER_TIMEOUT_SECONDS")
	if err != nil {
		dataForwarderTimeoutSeconds = defaultDataForwarderTimeoutSeconds
		Log.Warnf("Could not read ELASTIC_APM_DATA_FORWARDER_TIMEOUT_SECONDS, defaulting to %d: %v", dataForwarderTimeoutSeconds, err)
	}

	// add trailing slash to server name if missing
	normalizedApmLambdaServer := os.Getenv("ELASTIC_APM_LAMBDA_APM_SERVER")
	if normalizedApmLambdaServer != "" && normalizedApmLambdaServer[len(normalizedApmLambdaServer)-1:] != "/" {
		normalizedApmLambdaServer = normalizedApmLambdaServer + "/"
	}

	logLevel, err := logrus.ParseLevel(os.Getenv("ELASTIC_APM_LOG_LEVEL"))
	if err != nil {
		logLevel = logrus.InfoLevel
		Log.Warnf("Could not read ELASTIC_APM_LOG_LEVEL, defaulting to %s", logLevel)
	}

	// Get the send strategy, convert to lowercase
	normalizedSendStrategy := SyncFlush
	sendStrategy := strings.ToLower(os.Getenv("ELASTIC_APM_SEND_STRATEGY"))
	if sendStrategy == string(Background) {
		normalizedSendStrategy = Background
	}

	config := &extensionConfig{
		apmServerUrl:                normalizedApmLambdaServer,
		apmServerSecretToken:        os.Getenv("ELASTIC_APM_SECRET_TOKEN"),
		apmServerApiKey:             os.Getenv("ELASTIC_APM_API_KEY"),
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
		Log.Fatalln("please set ELASTIC_APM_LAMBDA_APM_SERVER, exiting")
	}
	if config.apmServerSecretToken == "" && config.apmServerApiKey == "" {
		Log.Fatalln("please set ELASTIC_APM_SECRET_TOKEN or ELASTIC_APM_API_KEY, exiting")
	}

	return config
}
