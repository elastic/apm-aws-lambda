package e2e_testing

import (
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"compress/zlib"
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
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var rebuildPtr = flag.Bool("rebuild", false, "rebuild lambda functions")
var langPtr = flag.String("lang", "node", "the language of the Lambda test function : Java, Node, or Python")
var timerPtr = flag.Int("timer", 20, "the timeout of the test lambda function")
var javaAgentVerPtr = flag.String("java-agent-ver", "1.28.4", "the version of the java APM agent")

func TestEndToEnd(t *testing.T) {

	// Check the only mandatory environment variable
	if err := godotenv.Load(".e2e_test_config"); err != nil {
		log.Println("No additional .e2e_test_config file found")
	}
	if getEnvVarValueOrSetDefault("RUN_E2E_TESTS", "false") != "true" {
		t.Skip("Skipping E2E tests. Please set the env. variable RUN_E2E_TESTS=true if you want to run them.")
	}

	supportedLanguages := []string{"node", "python", "java"}
	if !isStringInSlice(*langPtr, supportedLanguages) {
		processError(errors.New("unsupported language"))
	}

	samPath := "sam-" + *langPtr
	samServiceName := "sam-testing-" + *langPtr

	// Build and download required binaries (extension and Java agent)
	buildExtensionBinaries()

	// Java agent processing
	if *langPtr == "java" {
		if !folderExists(filepath.Join(samPath, "agent")) {
			log.Println("Java agent not found ! Collecting archive from Github...")
			retrieveJavaAgent(samPath, *javaAgentVerPtr)
		}
		changeJavaAgentPermissions(samPath)
	}

	// Initialize Mock APM Server
	mockAPMServerLog := ""
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/intake/v2/events" {
			bytesRes, _ := getDecompressedBytesFromRequest(r)
			mockAPMServerLog += string(bytesRes)
		}
	}))
	defer ts.Close()

	resultsChan := make(chan string, 1)

	uuid := runTestWithDedicatedTimer(samPath, samServiceName, ts.URL, *rebuildPtr, *timerPtr, resultsChan)
	if uuid == "" {
		t.Fail()
	}
	log.Printf("Querying the mock server for transaction bound to %s...", samServiceName)
	assert.True(t, strings.Contains(mockAPMServerLog, uuid))
}

func runTestWithDedicatedTimer(path string, serviceName string, serverURL string, buildFlag bool, timeout int, resultsChan chan string) string {
	timerNode := time.NewTimer(time.Duration(timeout) * time.Second * 2)
	go runTest(path, serviceName, serverURL, buildFlag, timeout, resultsChan)
	select {
	case uuid := <-resultsChan:
		return uuid
	case <-timerNode.C:
		break
	}
	return ""
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

	resultsChan <- uuidWithHyphen
}

func retrieveJavaAgent(samJavaPath string, version string) {

	agentFolderPath := filepath.Join(samJavaPath, "agent")
	agentArchivePath := filepath.Join(samJavaPath, "agent.zip")

	// Download archive
	out, err := os.Create(agentArchivePath)
	processError(err)
	defer out.Close()
	resp, err := http.Get(fmt.Sprintf("https://github.com/elastic/apm-agent-java/releases/download/v%[1]s/elastic-apm-java-aws-lambda-layer-%[1]s.zip", version))
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

func isStringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
