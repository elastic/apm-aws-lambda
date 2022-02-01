package e2e_testing

import (
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"compress/zlib"
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
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestEndToEnd(t *testing.T) {

	// Check the only mandatory environment variable
	if err := godotenv.Load(); err != nil {
		log.Println("No additional .env file found")
	}
	if getEnvVarValueOrSetDefault("RUN_E2E_TESTS", "false") != "true" {
		t.Skip("Skipping E2E tests. Please set the env. variable RUN_E2E_TESTS=true if you want to run them.")
	}

	forceBuildLambdasFlag := getEnvVarValueOrSetDefault("FORCE_REBUILD_LOCAL_SAM_LAMBDAS", "false") == "true"
	samNodePath := "sam-node"
	samNodeServiceName := "SamTestingNode"
	samPythonPath := "sam-python"
	samPythonServiceName := "SamTestingPython"
	samJavaPath := "sam-java"
	samJavaServiceName := "SamTestingJava"

	// Configure Timeouts and Tested Languages
	timeoutNode, err := strconv.Atoi(getEnvVarValueOrSetDefault("TIMEOUT_NODE", "20"))
	processError(err)
	timeoutPython, err := strconv.Atoi(getEnvVarValueOrSetDefault("TIMEOUT_PYTHON", "20"))
	processError(err)
	timeoutJava, err := strconv.Atoi(getEnvVarValueOrSetDefault("TIMEOUT_JAVA", "75"))
	processError(err)
	timeoutMax := getMax([]int{timeoutNode, timeoutPython, timeoutJava})

	// Build and download required binaries (extension and Java agent)
	buildExtensionBinaries()

	// Java agent processing
	if !folderExists(filepath.Join(samJavaPath, "agent")) {
		log.Println("Java agent not found ! Collecting archive from Github...")
		retrieveJavaAgent(samJavaPath)
	}
	changeJavaAgentPermissions(samJavaPath)

	// Initialize Mock APM Server
	mockAPMServerLog := ""
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/intake/v2/events" {
			bytesRes, _ := getDecompressedBytesFromRequest(r)
			mockAPMServerLog += string(bytesRes)
		}
	}))
	defer ts.Close()

	resultsChan := make(chan string, 3)
	resultsMap := make(map[string]string)

	if getEnvVarValueOrSetDefault("PARALLEL_EXECUTION", "true") == "true" {

		timer := time.NewTimer(time.Duration(timeoutMax) * time.Second)
		go runTest(samNodePath, samNodeServiceName, ts.URL, forceBuildLambdasFlag, timeoutNode, resultsChan)
		go runTest(samPythonPath, samPythonServiceName, ts.URL, forceBuildLambdasFlag, timeoutPython, resultsChan)
		go runTest(samJavaPath, samJavaServiceName, ts.URL, forceBuildLambdasFlag, timeoutJava, resultsChan)

	testLoop:
		for i := 0; i < 3; i++ {
			log.Println(i)
			select {
			case result := <-resultsChan:
				resultSlice := strings.Split(result, ":")
				switch resultSlice[0] {
				case samNodeServiceName:
					resultsMap["Node"] = resultSlice[1]
				case samPythonServiceName:
					resultsMap["Python"] = resultSlice[1]
				case samJavaServiceName:
					resultsMap["Java"] = resultSlice[1]
				}
			case <-timer.C:
				break testLoop
			}
		}

	} else {

		runTestWithDedicatedTimer(samNodePath, samNodeServiceName, ts.URL, forceBuildLambdasFlag, timeoutNode, resultsMap, resultsChan)
		runTestWithDedicatedTimer(samPythonPath, samPythonServiceName, ts.URL, forceBuildLambdasFlag, timeoutPython, resultsMap, resultsChan)
		runTestWithDedicatedTimer(samJavaPath, samJavaServiceName, ts.URL, forceBuildLambdasFlag, timeoutJava, resultsMap, resultsChan)

	}

	checkTestResults(samNodeServiceName, mockAPMServerLog, resultsMap, t)
	checkTestResults(samPythonServiceName, mockAPMServerLog, resultsMap, t)
	checkTestResults(samJavaServiceName, mockAPMServerLog, resultsMap, t)
}

func checkTestResults(serviceName string, serverLog string, resultsMap map[string]string, t *testing.T) {
	log.Printf("Querying the mock server for transaction bound to %s...", serviceName)
	if uuidNode, exists := resultsMap["Node"]; exists {
		assert.True(t, strings.Contains(serverLog, uuidNode))
	} else {
		t.Fail()
		log.Printf("FAILURE : Transaction %s bound to %s not found", uuidNode, serviceName)
	}
}

func runTestWithDedicatedTimer(path string, serviceName string, serverURL string, buildFlag bool, timeout int, resultsMap map[string]string, resultsChan chan string) {
	timerNode := time.NewTimer(time.Duration(timeout) * time.Second)
	runTest(path, serviceName, serverURL, buildFlag, timeout, resultsChan)
	log.Println("Ready to Select")
	select {
	case result := <-resultsChan:
		resultSlice := strings.Split(result, ":")
		resultsMap["Node"] = resultSlice[1]
	case <-timerNode.C:
		break
	}
}

func buildExtensionBinaries() {
	runCommandInDir("make", []string{}, "..", getEnvVarValueOrSetDefault("DEBUG_OUTPUT", "false") == "true")
}

func runTest(path string, serviceName string, serverURL string, buildFlag bool, timeout int, resultsChan chan string) {
	log.Printf("Starting to test %s", serviceName)

	if !folderExists(filepath.Join(path, ".aws-sam")) || buildFlag {
		log.Printf("Building the Lambda function %s", serviceName)
		runCommandInDir("sam", []string{"build"}, path, getEnvVarValueOrSetDefault("DEBUG_OUTPUT", "false") == "true")
	}

	log.Printf("Invoking the Lambda function %s", serviceName)
	uuidWithHyphen := uuid.New().String()
	urlSlice := strings.Split(serverURL, ":")
	port := urlSlice[len(urlSlice)-1]
	runCommandInDir("sam", []string{"local", "invoke", "--parameter-overrides",
		fmt.Sprintf("ParameterKey=ApmServerURL,ParameterValue=http://host.docker.internal:%s", port),
		fmt.Sprintf("ParameterKey=TestUUID,ParameterValue=%s", uuidWithHyphen),
		fmt.Sprintf("ParameterKey=TimeoutParam,ParameterValue=%d", timeout)},
		path, getEnvVarValueOrSetDefault("DEBUG_OUTPUT", "false") == "true")
	log.Printf("%s execution complete", serviceName)

	resultsChan <- fmt.Sprintf("%s:%s", serviceName, uuidWithHyphen)
}

func retrieveJavaAgent(samJavaPath string) {

	agentFolderPath := filepath.Join(samJavaPath, "agent")
	agentArchivePath := filepath.Join(samJavaPath, "agent.zip")

	// Download archive
	out, err := os.Create(agentArchivePath)
	processError(err)
	defer out.Close()
	resp, err := http.Get(fmt.Sprintf("https://github.com/elastic/apm-agent-java/releases/download/v%[1]s/elastic-apm-java-aws-lambda-layer-%[1]s.zip",
		getEnvVarValueOrSetDefault("APM_AGENT_JAVA_VERSION", "1.28.4")))
	processError(err)
	defer resp.Body.Close()
	io.Copy(out, resp.Body)

	// Unzip archive and delete it
	log.Println("Unzipping Java Agent archive...")
	unzip(agentArchivePath, agentFolderPath)
	err = os.Remove(agentArchivePath)
	processError(err)
}

func changeJavaAgentPermissions(samJavaPath string) {
	agentFolderPath := filepath.Join(samJavaPath, "agent")
	log.Println("Setting appropriate permissions for Java agent files...")
	agentFiles, err := ioutil.ReadDir(agentFolderPath)
	processError(err)
	for _, f := range agentFiles {
		os.Chmod(filepath.Join(agentFolderPath, f.Name()), 0755)
	}
}

func getEnvVarValueOrSetDefault(envVarName string, defaultVal string) string {
	val := os.Getenv(envVarName)
	if val == "" {
		return defaultVal
	}
	return val
}

func runCommandInDir(command string, args []string, dir string, printOutput bool) {
	e := exec.Command(command, args...)
	if printOutput {
		log.Println(e.String())
	}
	e.Dir = dir
	stdout, _ := e.StdoutPipe()
	stderr, _ := e.StderrPipe()
	e.Start()
	scannerOut := bufio.NewScanner(stdout)
	for scannerOut.Scan() {
		m := scannerOut.Text()
		if printOutput {
			log.Println(m)
		}
	}
	scannerErr := bufio.NewScanner(stderr)
	for scannerErr.Scan() {
		m := scannerErr.Text()
		if printOutput {
			log.Println(m)
		}
	}
	e.Wait()

}

func folderExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	return false
}

func processError(err error) {
	if err != nil {
		log.Panic(err)
	}
}

func unzip(archivePath string, destinationFolderPath string) {

	openedArchive, err := zip.OpenReader(archivePath)
	processError(err)
	defer openedArchive.Close()

	// Permissions setup
	os.MkdirAll(destinationFolderPath, 0755)

	// Closure required, so that Close() calls do not pile up when unzipping archives with a lot of files
	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			if err := rc.Close(); err != nil {
				panic(err)
			}
		}()

		path := filepath.Join(destinationFolderPath, f.Name)

		// Check for ZipSlip (Directory traversal)
		if !strings.HasPrefix(path, filepath.Clean(destinationFolderPath)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", path)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			os.MkdirAll(filepath.Dir(path), f.Mode())
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			processError(err)
			defer f.Close()
			_, err = io.Copy(f, rc)
			processError(err)
		}
		return nil
	}

	for _, f := range openedArchive.File {
		err := extractAndWriteFile(f)
		processError(err)
	}
}

func getDecompressedBytesFromRequest(req *http.Request) ([]byte, error) {
	var rawBytes []byte
	if req.Body != nil {
		rawBytes, _ = ioutil.ReadAll(req.Body)
	}

	switch req.Header.Get("Content-Encoding") {
	case "deflate":
		reader := bytes.NewReader([]byte(rawBytes))
		zlibreader, err := zlib.NewReader(reader)
		if err != nil {
			return nil, fmt.Errorf("could not create zlib.NewReader: %v", err)
		}
		bodyBytes, err := ioutil.ReadAll(zlibreader)
		if err != nil {
			return nil, fmt.Errorf("could not read from zlib reader using ioutil.ReadAll: %v", err)
		}
		return bodyBytes, nil
	case "gzip":
		reader := bytes.NewReader([]byte(rawBytes))
		zlibreader, err := gzip.NewReader(reader)
		if err != nil {
			return nil, fmt.Errorf("could not create gzip.NewReader: %v", err)
		}
		bodyBytes, err := ioutil.ReadAll(zlibreader)
		if err != nil {
			return nil, fmt.Errorf("could not read from gzip reader using ioutil.ReadAll: %v", err)
		}
		return bodyBytes, nil
	default:
		return rawBytes, nil
	}
}

func getMax(args []int) int {
	currMax := 0
	for _, el := range args {
		if el > currMax {
			currMax = el
		}
	}
	return currMax
}
