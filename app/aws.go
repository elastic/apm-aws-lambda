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
	"encoding/base64"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"go.uber.org/zap"
)

func loadAWSOptions(ctx context.Context, lazyCfg func() (*aws.Config, error), logger *zap.SugaredLogger) (string, string, error) {
	var manager *secretsmanager.Client
	lazyManager := func() (*secretsmanager.Client, error) {
		if manager != nil {
			return manager, nil
		}

		cfg, err := lazyCfg()
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS default config: %w", err)
		}

		manager = secretsmanager.NewFromConfig(*cfg)
		return manager, nil
	}

	apmServerApiKey := os.Getenv("ELASTIC_APM_API_KEY")
	if apmServerApiKeySMSecretId, ok := os.LookupEnv("ELASTIC_APM_SECRETS_MANAGER_API_KEY_ID"); ok {
		result, err := loadSecret(ctx, lazyManager, apmServerApiKeySMSecretId)
		if err != nil {
			logger.Warnf("Could not load APM API key from AWS Secrets Manager. Reporting APM data will likely fail. Is 'ELASTIC_APM_SECRETS_MANAGER_API_KEY_ID=%s' correct? See https://www.elastic.co/guide/en/apm/lambda/current/aws-lambda-secrets-manager.html. Error message: %v", apmServerApiKeySMSecretId, err)
			apmServerApiKey = ""
		} else {
			logger.Infof("Using the APM API key retrieved from AWS Secrets Manager.")
			apmServerApiKey = result
		}
	}

	apmServerSecretToken := os.Getenv("ELASTIC_APM_SECRET_TOKEN")
	if apmServerSecretTokenSMSecretId, ok := os.LookupEnv("ELASTIC_APM_SECRETS_MANAGER_SECRET_TOKEN_ID"); ok {
		result, err := loadSecret(ctx, lazyManager, apmServerSecretTokenSMSecretId)
		if err != nil {
			logger.Warnf("Could not load APM secret token from AWS Secrets Manager. Reporting APM data will likely fail. Is 'ELASTIC_APM_SECRETS_MANAGER_SECRET_TOKEN_ID=%s' correct? See https://www.elastic.co/guide/en/apm/lambda/current/aws-lambda-secrets-manager.html. Error message: %v", apmServerSecretTokenSMSecretId, err)
			apmServerSecretToken = ""
		} else {
			logger.Infof("Using the APM secret token retrieved from AWS Secrets Manager.")
			apmServerSecretToken = result
		}
	}

	return apmServerApiKey, apmServerSecretToken, nil
}

func loadSecret(ctx context.Context, lazyManager func() (*secretsmanager.Client, error), secretID string) (string, error) {
	input := &secretsmanager.GetSecretValueInput{
		SecretId:     ptrFromString(secretID),
		VersionStage: ptrFromString("AWSCURRENT"),
	}

	manager, err := lazyManager()
	if err != nil {
		return "", fmt.Errorf("failed to create manager: %w", err)
	}

	result, err := manager.GetSecretValue(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve sercet value: %w", err)
	}

	if result.SecretString != nil {
		return *result.SecretString, nil
	}

	decodedBinarySecretBytes := make([]byte, base64.StdEncoding.DecodedLen(len(result.SecretBinary)))
	if _, err := base64.StdEncoding.Decode(decodedBinarySecretBytes, result.SecretBinary); err != nil {
		return "", fmt.Errorf("failed to decode base64 encoded secret: %w", err)
	}

	return string(decodedBinarySecretBytes), nil
}

func loadAcmCertificate(arn string, lazyCfg func() (*aws.Config, error), ctx context.Context) (*string, error) {
	cfg, err := lazyCfg()
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS default config: %w", err)
	}
	acmClient := acm.NewFromConfig(*cfg)
	getCertificateInput := acm.GetCertificateInput{
		CertificateArn: &arn,
	}
	response, err := acmClient.GetCertificate(ctx, &getCertificateInput)
	if err != nil {
		return nil, err
	}

	return response.Certificate, nil
}

func ptrFromString(v string) *string {
	return &v
}
