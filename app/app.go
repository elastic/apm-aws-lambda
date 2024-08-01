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
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/apm-aws-lambda/accumulator"
	"github.com/elastic/apm-aws-lambda/apmproxy"
	"github.com/elastic/apm-aws-lambda/extension"
	"github.com/elastic/apm-aws-lambda/logger"
	"github.com/elastic/apm-aws-lambda/logsapi"

	"go.elastic.co/ecszap"
	"go.uber.org/zap"
)

const (
	defaultMaxBatchSize = 50
	defaultMaxBatchAge  = 2 * time.Second
)

// App is the main application.
type App struct {
	extensionName   string
	extensionClient *extension.Client
	logsClient      *logsapi.Client
	apmClient       *apmproxy.Client
	logger          *zap.SugaredLogger
	batch           *accumulator.Batch
}

// New returns an App or an error if the creation failed.
//
//nolint:govet
func New(ctx context.Context, opts ...ConfigOption) (*App, error) {
	c := appConfig{}

	for _, opt := range opts {
		opt(&c)
	}

	app := &App{
		extensionName: c.extensionName,
		batch:         accumulator.NewBatch(defaultMaxBatchSize, defaultMaxBatchAge),
	}

	var err error

	if app.logger, err = buildLogger(c.logLevel); err != nil {
		return nil, err
	}

	apmServerAPIKey, apmServerSecretToken := loadAWSOptions(ctx, c.awsConfig, app.logger)

	app.extensionClient = extension.NewClient(c.awsLambdaRuntimeAPI, app.logger)

	if !c.disableLogsAPI {
		addr := "sandbox.localdomain:0"
		if c.logsapiAddr != "" {
			addr = c.logsapiAddr
		}

		subscriptionLogStreams := []logsapi.SubscriptionType{logsapi.Platform}
		if c.enableFunctionLogSubscription {
			subscriptionLogStreams = append(subscriptionLogStreams, logsapi.Function)
		}

		app.logsClient, err = logsapi.NewClient(
			logsapi.WithLogsAPIBaseURL("http://"+c.awsLambdaRuntimeAPI),
			logsapi.WithListenerAddress(addr),
			logsapi.WithLogBuffer(100),
			logsapi.WithLogger(app.logger),
			logsapi.WithSubscriptionTypes(subscriptionLogStreams...),
			logsapi.WithInvocationLifecycler(app.batch),
		)
		if err != nil {
			return nil, err
		}
	}

	var apmOpts []apmproxy.Option

	if receiverTimeout, ok, err := parseDurationTimeout(app.logger, "ELASTIC_APM_DATA_RECEIVER_TIMEOUT", "ELASTIC_APM_DATA_RECEIVER_TIMEOUT_SECONDS"); err != nil || ok {
		if err != nil {
			return nil, err
		}
		apmOpts = append(apmOpts, apmproxy.WithReceiverTimeout(receiverTimeout))
	}

	if dataForwarderTimeout, ok, err := parseDurationTimeout(app.logger, "ELASTIC_APM_DATA_FORWARDER_TIMEOUT", "ELASTIC_APM_DATA_FORWARDER_TIMEOUT_SECONDS"); err != nil || ok {
		if err != nil {
			return nil, err
		}
		apmOpts = append(apmOpts, apmproxy.WithDataForwarderTimeout(dataForwarderTimeout))
	}

	if port := os.Getenv("ELASTIC_APM_DATA_RECEIVER_SERVER_PORT"); port != "" {
		apmOpts = append(apmOpts, apmproxy.WithReceiverAddress(":"+port))
	}

	if strategy, ok := parseStrategy(os.Getenv("ELASTIC_APM_SEND_STRATEGY")); ok {
		apmOpts = append(apmOpts, apmproxy.WithSendStrategy(strategy))
	}

	if bufferSize := os.Getenv("ELASTIC_APM_LAMBDA_AGENT_DATA_BUFFER_SIZE"); bufferSize != "" {
		size, err := strconv.Atoi(bufferSize)
		if err != nil {
			return nil, err
		}

		apmOpts = append(apmOpts, apmproxy.WithAgentDataBufferSize(size))
	}

	if verifyCertsString := os.Getenv("ELASTIC_APM_LAMBDA_VERIFY_SERVER_CERT"); verifyCertsString != "" {
		verifyCerts, err := strconv.ParseBool(verifyCertsString)
		if err != nil {
			return nil, err
		}
		if !verifyCerts {
			app.logger.Infof("Ignoring Certificates.")
		}
		apmOpts = append(apmOpts, apmproxy.WithVerifyCerts(verifyCerts))
	}

	if encodedCertPem := os.Getenv("ELASTIC_APM_LAMBDA_SERVER_CA_CERT_PEM"); encodedCertPem != "" {
		certPem := strings.ReplaceAll(encodedCertPem, "\\n", "\n")
		app.logger.Infof("Using CA certificates from environment variable.")
		apmOpts = append(apmOpts, apmproxy.WithRootCerts(certPem))
	}

	if certFile := os.Getenv("ELASTIC_APM_SERVER_CA_CERT_FILE"); certFile != "" {
		cert, err := os.ReadFile(certFile)
		if err != nil {
			return nil, err
		}
		app.logger.Infof("Using CA certificate loaded from file %s", certFile)
		apmOpts = append(apmOpts, apmproxy.WithRootCerts(string(cert)))
	}

	if acmCertArn := os.Getenv("ELASTIC_APM_SERVER_CA_CERT_ACM_ID"); acmCertArn != "" {
		cert, err := loadAcmCertificate(ctx, acmCertArn, c.awsConfig)
		if err != nil {
			return nil, err
		}
		app.logger.Infof("Using CA certificate %s", acmCertArn)
		apmOpts = append(apmOpts, apmproxy.WithRootCerts(*cert))
	}

	apmOpts = append(apmOpts,
		apmproxy.WithURL(os.Getenv("ELASTIC_APM_LAMBDA_APM_SERVER")),
		apmproxy.WithLogger(app.logger),
		apmproxy.WithAPIKey(apmServerAPIKey),
		apmproxy.WithSecretToken(apmServerSecretToken),
		apmproxy.WithBatch(app.batch),
	)

	ac, err := apmproxy.NewClient(apmOpts...)

	if err != nil {
		return nil, err
	}

	app.apmClient = ac

	return app, nil
}

func parseDurationTimeout(l *zap.SugaredLogger, flag string, deprecatedFlag string) (time.Duration, bool, error) {
	if strValue, ok := os.LookupEnv(flag); ok {
		d, err := time.ParseDuration(strValue)
		if err != nil {
			return 0, false, fmt.Errorf("failed to parse %s: %w", flag, err)
		}

		return d, true, nil
	}

	if strValueSeconds, ok := os.LookupEnv(deprecatedFlag); ok {
		l.Warnf("%s is deprecated, please consider moving to %s", deprecatedFlag, flag)

		seconds, err := strconv.Atoi(strValueSeconds)
		if err != nil {
			return 0, false, fmt.Errorf("failed to parse %s: %w", deprecatedFlag, err)
		}

		return time.Duration(seconds) * time.Second, true, nil
	}

	return 0, false, nil
}

func parseStrategy(value string) (apmproxy.SendStrategy, bool) {
	switch strings.ToLower(value) {
	case "background":
		return apmproxy.Background, true
	case "syncflush":
		return apmproxy.SyncFlush, true
	}

	return "", false
}

func buildLogger(level string) (*zap.SugaredLogger, error) {
	if level == "" {
		level = "info"
	}

	l, err := logger.ParseLogLevel(level)
	if err != nil {
		return nil, err
	}

	return logger.New(
		logger.WithEncoderConfig(ecszap.NewDefaultEncoderConfig().ToZapCoreEncoderConfig()),
		logger.WithLevel(l),
	)
}
