package e2e_testing

import (
	"errors"
	"flag"
	"fmt"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var rebuildPtr = flag.Bool("rebuild", false, "rebuild lambda functions")
var langPtr = flag.String("lang", "nodejs", "the language of the Lambda test function : Java, Node, or Python")
var timerPtr = flag.Int("timer", 20, "the timeout of the test lambda function")
var javaAgentVerPtr = flag.String("java-agent-ver", "1.28.4", "the version of the java APM agent")

func TestEndToEnd(t *testing.T) {

	// Check the only mandatory environment variable
	if err := godotenv.Load(".e2e_test_config"); err != nil {
		log.Println("No additional .e2e_test_config file found")
	}
	if GetEnvVarValueOrSetDefault("RUN_E2E_TESTS", "false") != "true" {
		t.Skip("Skipping E2E tests. Please set the env. variable RUN_E2E_TESTS=true if you want to run them.")
	}

	languageName := strings.ToLower(*langPtr)
	supportedLanguages := []string{"nodejs", "python", "java"}
	if !IsStringInSlice(languageName, supportedLanguages) {
		ProcessError(errors.New(fmt.Sprintf("Unsupported language %s ! Supported languages are %v", languageName, supportedLanguages)))
	}

	samPath := "sam-" + languageName
	samServiceName := "sam-testing-" + languageName

	// Build and download required binaries (extension and Java agent)
	buildExtensionBinaries()

	// Java agent processing
	if languageName == "java" {
		if !FolderExists(filepath.Join(samPath, "agent")) {
			log.Println("Java agent not found ! Collecting archive from Github...")
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

	uuid := runTestWithTimer(samPath, samServiceName, ts.URL, *rebuildPtr, *timerPtr, resultsChan)
	log.Printf("UUID generated during the test : %s", uuid)
	if uuid == "" {
		t.Fail()
	}
	log.Printf("Querying the mock server for transaction bound to %s...", samServiceName)
	assert.True(t, strings.Contains(mockAPMServerLog, uuid))
}

func runTestWithTimer(path string, serviceName string, serverURL string, buildFlag bool, lambdaFuncTimeout int, resultsChan chan string) string {
	timer := time.NewTimer(time.Duration(lambdaFuncTimeout) * time.Second * 2)
	defer timer.Stop()
	go runTest(path, serviceName, serverURL, buildFlag, lambdaFuncTimeout, resultsChan)
	select {
	case uuid := <-resultsChan:
		return uuid
	case <-timer.C:
		return ""
	}
}

func buildExtensionBinaries() {
	RunCommandInDir("make", []string{}, "..", GetEnvVarValueOrSetDefault("DEBUG_OUTPUT", "false") == "true")
}

func runTest(path string, serviceName string, serverURL string, buildFlag bool, lambdaFuncTimeout int, resultsChan chan string) {
	log.Printf("Starting to test %s", serviceName)

	if !FolderExists(filepath.Join(path, ".aws-sam")) || buildFlag {
		log.Printf("Building the Lambda function %s", serviceName)
		RunCommandInDir("sam", []string{"build"}, path, GetEnvVarValueOrSetDefault("DEBUG_OUTPUT", "false") == "true")
	}

	log.Printf("Invoking the Lambda function %s", serviceName)
	uuidWithHyphen := uuid.New().String()
	urlSlice := strings.Split(serverURL, ":")
	port := urlSlice[len(urlSlice)-1]
	RunCommandInDir("sam", []string{"local", "invoke", "--parameter-overrides",
		fmt.Sprintf("ParameterKey=ApmServerURL,ParameterValue=http://host.docker.internal:%s", port),
		fmt.Sprintf("ParameterKey=TestUUID,ParameterValue=%s", uuidWithHyphen),
		fmt.Sprintf("ParameterKey=TimeoutParam,ParameterValue=%d", lambdaFuncTimeout)},
		path, GetEnvVarValueOrSetDefault("DEBUG_OUTPUT", "false") == "true")
	log.Printf("%s execution complete", serviceName)

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
	io.Copy(out, resp.Body)

	// Unzip archive and delete it
	log.Println("Unzipping Java Agent archive...")
	Unzip(agentArchivePath, agentFolderPath)
	err = os.Remove(agentArchivePath)
	ProcessError(err)
}

func changeJavaAgentPermissions(samJavaPath string) {
	agentFolderPath := filepath.Join(samJavaPath, "agent")
	log.Println("Setting appropriate permissions for Java agent files...")
	agentFiles, err := ioutil.ReadDir(agentFolderPath)
	ProcessError(err)
	for _, f := range agentFiles {
		os.Chmod(filepath.Join(agentFolderPath, f.Name()), 0755)
	}
}
