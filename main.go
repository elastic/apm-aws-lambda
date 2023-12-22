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
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/elastic/apm-aws-lambda/app"
)

func main() {
	// Global context
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	appConfigs := []app.ConfigOption{
		app.WithExtensionName(filepath.Base(os.Args[0])),
		app.WithLambdaRuntimeAPI(os.Getenv("AWS_LAMBDA_RUNTIME_API")),
		app.WithLogLevel(os.Getenv("ELASTIC_APM_LOG_LEVEL")),
	}

	// ELASTIC_APM_LAMBDA_CAPTURE_LOGS indicate if the lambda extension
	// should capture logs, the value defaults to true i.e. the extension
	// will capture function logs by default
	if rawLambdaCaptureLogs, ok := os.LookupEnv("ELASTIC_APM_LAMBDA_CAPTURE_LOGS"); ok {
		if captureLogs, err := strconv.ParseBool(rawLambdaCaptureLogs); err != nil {
			appConfigs = append(appConfigs, app.WithFunctionLogSubscription(captureLogs))
		}
	}

	app, err := app.New(ctx, appConfigs...)
	if err != nil {
		log.Fatalf("failed to create the app: %v", err)
	}

	if err := app.Run(ctx); err != nil {
		log.Fatalf("error while running: %v", err)
	}
}
