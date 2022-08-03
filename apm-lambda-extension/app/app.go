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

package app

import (
	"elastic/apm-lambda-extension/apmproxy"
	"elastic/apm-lambda-extension/extension"
	"elastic/apm-lambda-extension/logsapi"
	"fmt"
	"os"
	"strconv"
	"time"
)

// App is the main application.
type App struct {
	extensionName   string
	extensionClient *extension.Client
	logsClient      *logsapi.Client
	apmClient       *apmproxy.Client
}

// New returns an App or an error if the
// creation failed.
func New(opts ...configOption) (*App, error) {
	c := appConfig{}

	for _, opt := range opts {
		opt(&c)
	}

	app := &App{
		extensionName:   c.extensionName,
		extensionClient: extension.NewClient(c.awsLambdaRuntimeAPI),
	}

	if !c.disableLogsAPI {
		lc, err := logsapi.NewClient(
			logsapi.WithLogsAPIBaseURL(fmt.Sprintf("http://%s", c.awsLambdaRuntimeAPI)),
			logsapi.WithListenerAddress("sandbox:0"),
			logsapi.WithLogBuffer(100),
		)
		if err != nil {
			return nil, err
		}

		app.logsClient = lc
	}

	var apmOpts []apmproxy.Option

	receiverTimeoutSeconds, err := getIntFromEnvIfAvailable("ELASTIC_APM_DATA_RECEIVER_TIMEOUT_SECONDS")
	if err != nil {
		return nil, err
	}
	if receiverTimeoutSeconds != 0 {
		apmOpts = append(apmOpts, apmproxy.WithReceiverTimeout(time.Duration(receiverTimeoutSeconds)*time.Second))
	}

	dataForwarderTimeoutSeconds, err := getIntFromEnvIfAvailable("ELASTIC_APM_DATA_FORWARDER_TIMEOUT_SECONDS")
	if err != nil {
		return nil, err
	}
	if dataForwarderTimeoutSeconds != 0 {
		apmOpts = append(apmOpts, apmproxy.WithDataForwarderTimeout(time.Duration(dataForwarderTimeoutSeconds)*time.Second))
	}

	if port := os.Getenv("ELASTIC_APM_DATA_RECEIVER_SERVER_PORT"); port != "" {
		apmOpts = append(apmOpts, apmproxy.WithReceiverAddress(fmt.Sprintf(":%s", port)))
	}

	if bufferSize := os.Getenv("ELASTIC_APM_LAMBDA_AGENT_DATA_BUFFER_SIZE"); bufferSize != "" {
		size, err := strconv.Atoi(bufferSize)
		if err != nil {
			return nil, err
		}

		apmOpts = append(apmOpts, apmproxy.WithAgentDataBufferSize(size))
	}

	apmOpts = append(apmOpts, apmproxy.WithURL(os.Getenv("ELASTIC_APM_LAMBDA_APM_SERVER")))

	ac, err := apmproxy.NewClient(apmOpts...)

	if err != nil {
		return nil, err
	}

	app.apmClient = ac

	return app, nil
}

func getIntFromEnvIfAvailable(name string) (int, error) {
	strValue := os.Getenv(name)
	if strValue == "" {
		return 0, nil
	}

	value, err := strconv.Atoi(strValue)
	if err != nil {
		return -1, err
	}
	return value, nil
}
