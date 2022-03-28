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

package e2e_testing

import (
	"elastic/apm-lambda-extension/extension"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
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

	if os.Getenv("ELASTIC_APM_LOG_LEVEL") != "" {
		logLevel, _ := logrus.ParseLevel(os.Getenv("ELASTIC_APM_LOG_LEVEL"))
		extension.Log.Logger.SetLevel(logLevel)
	}
	if GetEnvVarValueOrSetDefault("RUN_E2E_TESTS", "false") != "true" {
		t.Skip("Skipping E2E tests. Please set the env. variable RUN_E2E_TESTS=true if you want to run them.")
	}

	extension.Log.Info("If the end-to-end tests are failing unexpectedly, please verify that Docker is running on your machine.")

	languageName := strings.ToLower(*langPtr)
	supportedLanguages := []string{"nodejs", "python", "java"}
	if !IsStringInSlice(languageName, supportedLanguages) {
		ProcessError(fmt.Errorf(fmt.Sprintf("Unsupported language %s ! Supported languages are %v", languageName, supportedLanguages)))
	}

	samPath := "sam-" + languageName
	samServiceName := "sam-testing-" + languageName

	// Build and download required binaries (extension and Java agent)
	buildExtensionBinaries()

	// Java agent processing
	if languageName == "java" {
		if !FolderExists(filepath.Join(samPath, "agent")) {
			extension.Log.Warn("Java agent not found ! Collecting archive from Github...")
			retrieveJavaAgent(samPath, *javaAgentVerPtr)
		}
		changeJavaAgentPermissions(samPath)
	}

	// Initialize Mock APM Server
	mockAPMServerLog := ""
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/intake/v2/events" {
			bytesRes, _ := GetDecompressedBytesFromRequest(r)
			mockAPMServerLog += fmt.Sprintf("%s\n", bytesRes)
		}
	}))
	defer ts.Close()

	resultsChan := make(chan string, 1)

	testUuid := runTestWithTimer(samPath, samServiceName, ts.URL, *rebuildPtr, *timerPtr, resultsChan)
	extension.Log.Infof("UUID generated during the test : %s", testUuid)
	if testUuid == "" {
		t.Fail()
	}
	extension.Log.Infof("Querying the mock server for transaction bound to %s...", samServiceName)
	assert.True(t, strings.Contains(mockAPMServerLog, testUuid))
}

func runTestWithTimer(path string, serviceName string, serverURL string, buildFlag bool, lambdaFuncTimeout int, resultsChan chan string) string {
	timer := time.NewTimer(time.Duration(lambdaFuncTimeout) * time.Second * 2)
	defer timer.Stop()
	go runTest(path, serviceName, serverURL, buildFlag, lambdaFuncTimeout, resultsChan)
	select {
	case testUuid := <-resultsChan:
		return testUuid
	case <-timer.C:
		return ""
	}
}

func buildExtensionBinaries() {
	RunCommandInDir("make", []string{}, "..")
}

func runTest(path string, serviceName string, serverURL string, buildFlag bool, lambdaFuncTimeout int, resultsChan chan string) {
	extension.Log.Infof("Starting to test %s", serviceName)

	if !FolderExists(filepath.Join(path, ".aws-sam")) || buildFlag {
		extension.Log.Infof("Building the Lambda function %s", serviceName)
		RunCommandInDir("sam", []string{"build"}, path)
	}

	extension.Log.Infof("Invoking the Lambda function %s", serviceName)
	uuidWithHyphen := uuid.New().String()
	urlSlice := strings.Split(serverURL, ":")
	port := urlSlice[len(urlSlice)-1]
	RunCommandInDir("sam", []string{"local", "invoke", "--parameter-overrides",
		fmt.Sprintf("ParameterKey=ApmServerURL,ParameterValue=http://host.docker.internal:%s", port),
		fmt.Sprintf("ParameterKey=TestUUID,ParameterValue=%s", uuidWithHyphen),
		fmt.Sprintf("ParameterKey=TimeoutParam,ParameterValue=%d", lambdaFuncTimeout)},
		path)
	extension.Log.Infof("%s execution complete", serviceName)

	resultsChan <- uuidWithHyphen
}

func retrieveJavaAgent(samJavaPath string, version string) {

	agentFolderPath := filepath.Join(samJavaPath, "agent")
	agentArchivePath := filepath.Join(samJavaPath, "agent.zip")

	// Download archive
	out, err := os.Create(agentArchivePath)
	ProcessError(err)
	defer out.Close()
	resp, err := http.Get(fmt.Sprintf("https://github.com/elastic/apm-agent-java/releases/download/v%[1]s/elastic-apm-java-aws-lambda-layer-%[1]s.zip", version))
	ProcessError(err)
	defer resp.Body.Close()
	if _, err = io.Copy(out, resp.Body); err != nil {
		extension.Log.Errorf("Could not retrieve java agent : %v", err)
	}

	// Unzip archive and delete it
	extension.Log.Info("Unzipping Java Agent archive...")
	Unzip(agentArchivePath, agentFolderPath)
	err = os.Remove(agentArchivePath)
	ProcessError(err)
}

func changeJavaAgentPermissions(samJavaPath string) {
	agentFolderPath := filepath.Join(samJavaPath, "agent")
	extension.Log.Info("Setting appropriate permissions for Java agent files...")
	agentFiles, err := ioutil.ReadDir(agentFolderPath)
	ProcessError(err)
	for _, f := range agentFiles {
		if err = os.Chmod(filepath.Join(agentFolderPath, f.Name()), 0755); err != nil {
			extension.Log.Errorf("Could not change java agent permissions : %v", err)
		}
	}
}
