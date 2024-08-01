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

package e2etesting

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/elastic/apm-aws-lambda/logger"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var rebuildPtr = flag.Bool("rebuild", false, "rebuild lambda functions")
var langPtr = flag.String("lang", "nodejs", "the language of the Lambda test function : Java, Node, or Python")
var timerPtr = flag.Int("timer", 20, "the timeout of the test lambda function")
var javaAgentVerPtr = flag.String("java-agent-ver", "1.28.4", "the version of the java APM agent")

func TestEndToEnd(t *testing.T) {
	// Check the only mandatory environment variable
	if err := godotenv.Load(".e2e_test_config"); err != nil {
		panic("No config file")
	}

	logLevel, err := logger.ParseLogLevel(os.Getenv("ELASTIC_APM_LOG_LEVEL"))
	require.NoError(t, err)

	l, err := logger.New(
		logger.WithLevel(logLevel),
	)
	require.NoError(t, err)

	if GetEnvVarValueOrSetDefault("RUN_E2E_TESTS", "false") != "true" {
		t.Skip("Skipping E2E tests. Please set the env. variable RUN_E2E_TESTS=true if you want to run them.")
	}

	l.Info("If the end-to-end tests are failing unexpectedly, please verify that Docker is running on your machine.")

	languageName := strings.ToLower(*langPtr)
	supportedLanguages := []string{"nodejs", "python", "java"}
	if !IsStringInSlice(languageName, supportedLanguages) {
		ProcessError(l, fmt.Errorf("unsupported language %s ! Supported languages are %v", languageName, supportedLanguages))
	}

	samPath := "sam-" + languageName
	samServiceName := "sam-testing-" + languageName

	// Build and download required binaries (extension and Java agent)
	buildExtensionBinaries(l)

	// Java agent processing
	if languageName == "java" {
		if !FolderExists(filepath.Join(samPath, "agent")) {
			l.Warn("Java agent not found ! Collecting archive from Github...")
			retrieveJavaAgent(l, samPath, *javaAgentVerPtr)
		}
		changeJavaAgentPermissions(l, samPath)
	}

	// Initialize Mock APM Server
	mockAPMServerLog := ""
	ts := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/intake/v2/events" {
			bytesRes, _ := GetDecompressedBytesFromRequest(r)
			mockAPMServerLog += fmt.Sprintf("%s\n", bytesRes)
		}
	}))
	defer ts.Close()

	resultsChan := make(chan string, 1)

	testUUID := runTestWithTimer(l, samPath, samServiceName, ts.URL, *rebuildPtr, *timerPtr, resultsChan)
	l.Infof("UUID generated during the test : %s", testUUID)
	if testUUID == "" {
		t.Fail()
	}
	l.Infof("Querying the mock server for transaction bound to %s...", samServiceName)
	assert.True(t, strings.Contains(mockAPMServerLog, testUUID))
}

func runTestWithTimer(l *zap.SugaredLogger, path string, serviceName string, serverURL string, buildFlag bool, lambdaFuncTimeout int, resultsChan chan string) string {
	timer := time.NewTimer(time.Duration(lambdaFuncTimeout) * time.Second * 2)
	defer timer.Stop()
	go runTest(l, path, serviceName, serverURL, buildFlag, lambdaFuncTimeout, resultsChan)
	select {
	case testUUID := <-resultsChan:
		return testUUID
	case <-timer.C:
		return ""
	}
}

func buildExtensionBinaries(l *zap.SugaredLogger) {
	RunCommandInDir(l, "make", []string{}, "..")
}

func runTest(l *zap.SugaredLogger, path string, serviceName string, serverURL string, buildFlag bool, lambdaFuncTimeout int, resultsChan chan string) {
	l.Infof("Starting to test %s", serviceName)

	if !FolderExists(filepath.Join(path, ".aws-sam")) || buildFlag {
		l.Infof("Building the Lambda function %s", serviceName)
		RunCommandInDir(l, "sam", []string{"build"}, path)
	}

	l.Infof("Invoking the Lambda function %s", serviceName)
	uuidWithHyphen := uuid.New().String()
	urlSlice := strings.Split(serverURL, ":")
	port := urlSlice[len(urlSlice)-1]
	RunCommandInDir(l, "sam", []string{"local", "invoke", "--parameter-overrides",
		"ParameterKey=ApmServerURL,ParameterValue=http://host.docker.internal:" + port,
		"ParameterKey=TestUUID,ParameterValue=" + uuidWithHyphen,
		"ParameterKey=TimeoutParam,ParameterValue=" + strconv.Itoa(lambdaFuncTimeout)},
		path)
	l.Infof("%s execution complete", serviceName)

	resultsChan <- uuidWithHyphen
}

func retrieveJavaAgent(l *zap.SugaredLogger, samJavaPath string, version string) {
	agentFolderPath := filepath.Join(samJavaPath, "agent")
	agentArchivePath := filepath.Join(samJavaPath, "agent.zip")

	// Download archive
	out, err := os.Create(agentArchivePath)
	ProcessError(l, err)
	defer out.Close()
	resp, err := http.Get(fmt.Sprintf("https://github.com/elastic/apm-agent-java/releases/download/v%[1]s/elastic-apm-java-aws-lambda-layer-%[1]s.zip", version))
	ProcessError(l, err)
	defer resp.Body.Close()
	if _, err = io.Copy(out, resp.Body); err != nil {
		l.Errorf("Could not retrieve java agent : %v", err)
	}

	// Unzip archive and delete it
	l.Info("Unzipping Java Agent archive...")
	Unzip(l, agentArchivePath, agentFolderPath)
	err = os.Remove(agentArchivePath)
	ProcessError(l, err)
}

func changeJavaAgentPermissions(l *zap.SugaredLogger, samJavaPath string) {
	agentFolderPath := filepath.Join(samJavaPath, "agent")
	l.Info("Setting appropriate permissions for Java agent files...")
	agentFiles, err := os.ReadDir(agentFolderPath)
	ProcessError(l, err)
	for _, f := range agentFiles {
		if err = os.Chmod(filepath.Join(agentFolderPath, f.Name()), 0755); err != nil {
			l.Errorf("Could not change java agent permissions : %v", err)
		}
	}
}
