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
	"syscall"

	"elastic/apm-lambda-extension/app"
)

func main() {
	// Global context
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	app, err := app.New(
		app.WithExtensionName(filepath.Base(os.Args[0])),
		app.WithLambdaRuntimeAPI(os.Getenv("AWS_LAMBDA_RUNTIME_API")),
		app.WithLogLevel(os.Getenv("ELASTIC_APM_LOG_LEVEL")),
	)
	if err != nil {
		log.Fatalf("failed to create the app: %v", err)
	}

	if err := app.Run(ctx); err != nil {
		log.Fatalf("error while running: %v", err)
	}
}
