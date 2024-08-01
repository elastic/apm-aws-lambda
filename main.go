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

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/elastic/apm-aws-lambda/app"
)

func main() {
	if err := mainWithError(); err != nil {
		log.Fatal(err)
	}
}

func mainWithError() error {
	// Global context
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS default config: %v", err)
	}

	appConfigs := []app.ConfigOption{
		app.WithExtensionName(filepath.Base(os.Args[0])),
		app.WithLambdaRuntimeAPI(os.Getenv("AWS_LAMBDA_RUNTIME_API")),
		app.WithLogLevel(os.Getenv("ELASTIC_APM_LOG_LEVEL")),
		app.WithAWSConfig(cfg),
	}

	rawDisableLogsAPI := os.Getenv("ELASTIC_APM_LAMBDA_DISABLE_LOGS_API")
	if disableLogsAPI, _ := strconv.ParseBool(rawDisableLogsAPI); disableLogsAPI {
		appConfigs = append(appConfigs, app.WithoutLogsAPI())
	}

	// ELASTIC_APM_LAMBDA_CAPTURE_LOGS indicate if the lambda extension
	// should capture logs, the value defaults to true i.e. the extension
	// will capture function logs by default
	rawLambdaCaptureLogs := os.Getenv("ELASTIC_APM_LAMBDA_CAPTURE_LOGS")
	captureLogs, err := strconv.ParseBool(rawLambdaCaptureLogs)
	if err != nil {
		if rawLambdaCaptureLogs != "" {
			log.Printf("failed to parse env var ELASTIC_APM_LAMBDA_CAPTURE_LOGS, defaulting to true")
		}
		captureLogs = true
	}

	if captureLogs {
		appConfigs = append(appConfigs, app.WithFunctionLogSubscription())
	}

	application, err := app.New(ctx, appConfigs...)
	if err != nil {
		return fmt.Errorf("failed to create the app: %v", err)
	}

	if err := application.Run(ctx); err != nil {
		return fmt.Errorf("error while running: %v", err)
	}

	return nil
}
