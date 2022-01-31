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
	"strings"
	"sync"
	"testing"
)

func TestEndToEnd(t *testing.T) {

	// Check the only mandatory environment variable
	if err := godotenv.Load(); err != nil {
		log.Println("No additional .env file found")
	}
	if os.Getenv("RUN_E2E_TESTS") != "true" {
		t.Skip("Skipping E2E tests. Please set the env. variable RUN_E2E_TESTS=true if you want to run them.")
	}

	forceBuildLambdasFlag := os.Getenv("FORCE_REBUILD_LOCAL_SAM_LAMBDAS") == "true"
	samNodePath := "sam-node"
	samPythonPath := "sam-python"
	samJavaPath := "sam-java"

	// Build and download required binaries (extension and Java agent)
	buildExtensionBinaries()
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

	if os.Getenv("PARALLEL_EXECUTION") == "true" {
		var waitGroup sync.WaitGroup
		waitGroup.Add(3)

		cNode := make(chan string)
		go runTestAsync(samNodePath, "SamTestingNode", ts.URL, forceBuildLambdasFlag, &waitGroup, cNode)
		cPython := make(chan string)
		go runTestAsync(samPythonPath, "SamTestingPython", ts.URL, forceBuildLambdasFlag, &waitGroup, cPython)
		cJava := make(chan string)
		go runTestAsync(samJavaPath, "SamTestingJava", ts.URL, forceBuildLambdasFlag, &waitGroup, cJava)
		// Retrieve the UUID generated by the tests though dedicated channels
		uuidNode := <-cNode
		uuidPython := <-cPython
		uuidJava := <-cJava
		log.Printf("Querying the mock server for transaction %s bound to %s...", uuidNode, "SamTestingNode")
		assert.True(t, strings.Contains(mockAPMServerLog, uuidNode))
		log.Printf("Querying the mock server for transaction %s bound to %s...", uuidPython, "SamTestingPython")
		assert.True(t, strings.Contains(mockAPMServerLog, uuidPython))
		log.Printf("Querying the mock server for transaction %s bound to %s...", uuidJava, "SamTestingJava")
		assert.True(t, strings.Contains(mockAPMServerLog, uuidJava))
		waitGroup.Wait()

	} else {
		uuidNode := runTest(samNodePath, "SamTestingNode", ts.URL, forceBuildLambdasFlag)
		uuidPython := runTest(samPythonPath, "SamTestingPython", ts.URL, forceBuildLambdasFlag)
		uuidJava := runTest(samJavaPath, "SamTestingJava", ts.URL, forceBuildLambdasFlag)
		log.Printf("Querying the mock server logs for transaction %s bound to %s...", uuidNode, "SamTestingNode")
		assert.True(t, strings.Contains(mockAPMServerLog, uuidNode))
		log.Printf("Querying the mock server logs for transaction %s bound to %s...", uuidPython, "SamTestingPython")
		assert.True(t, strings.Contains(mockAPMServerLog, uuidPython))
		log.Printf("Querying the mock server logs for transaction %s bound to %s...", uuidJava, "SamTestingJava")
		assert.True(t, strings.Contains(mockAPMServerLog, uuidJava))
	}
}

func buildExtensionBinaries() {
	runCommandInDir("make", []string{}, "..", os.Getenv("DEBUG_OUTPUT") == "true")
}

func runTest(path string, serviceName string, serverURL string, buildFlag bool) string {
	log.Printf("Starting to test %s", serviceName)

	if !folderExists(filepath.Join(path, ".aws-sam")) || buildFlag {
		log.Printf("Building the Lambda function %s", serviceName)
		runCommandInDir("sam", []string{"build"}, path, os.Getenv("DEBUG_OUTPUT") == "true")
	}

	log.Printf("Invoking the Lambda function %s", serviceName)
	uuidWithHyphen := uuid.New().String()
	urlSlice := strings.Split(serverURL, ":")
	port := urlSlice[len(urlSlice)-1]
	runCommandInDir("sam", []string{"local", "invoke", "--parameter-overrides",
		fmt.Sprintf("ParameterKey=ApmServerURL,ParameterValue=http://host.docker.internal:%s", port),
		fmt.Sprintf("ParameterKey=TestUUID,ParameterValue=%s", uuidWithHyphen)},
		path, os.Getenv("DEBUG_OUTPUT") == "true")
	log.Printf("%s execution complete", serviceName)

	return uuidWithHyphen
}

func runTestAsync(path string, serviceName string, serverURL string, buildFlag bool, wg *sync.WaitGroup, c chan string) {
	defer wg.Done()
	out := runTest(path, serviceName, serverURL, buildFlag)
	c <- out
}

func retrieveJavaAgent(samJavaPath string) {

	agentFolderPath := filepath.Join(samJavaPath, "agent")
	agentArchivePath := filepath.Join(samJavaPath, "agent.zip")

	// Download archive
	out, err := os.Create(agentArchivePath)
	processError(err)
	defer out.Close()
	resp, err := http.Get(fmt.Sprintf("https://github.com/elastic/apm-agent-java/releases/download/v%[1]s/elastic-apm-java-aws-lambda-layer-%[1]s.zip",
		os.Getenv("APM_AGENT_JAVA_VERSION")))
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