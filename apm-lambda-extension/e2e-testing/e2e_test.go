package e2e_testing

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestEndToEndExtensionBehavior(t *testing.T) {

	if err := godotenv.Load(); err != nil {
		t.Skip("No .env file found alongside e2e_test.go : Skipping end-to-end tests.")
	}
	if os.Getenv("RUN_E2E_TESTS") != "true" {
		t.Skip("Skipping E2E tests. Please set the env. variable RUN_E2E_TESTS=true if you want to run them.")
	}
	if os.Getenv("ELASTIC_CLOUD_ID") == "" {
		log.Println("Please set the ELASTIC_CLOUD_ID environment variable")
		os.Exit(1)
	}
	if os.Getenv("ELASTIC_CLOUD_API_KEY") == "" {
		log.Println("Please set the ELASTIC_CLOUD_API_KEY environment variable")
		os.Exit(1)
	}
	if os.Getenv("ELASTIC_APM_SERVER_URL") == "" {
		log.Println("Please set the ELASTIC_APM_SERVER_URL environment variable")
		os.Exit(1)
	}
	if os.Getenv("ELASTIC_APM_SERVER_TOKEN") == "" {
		log.Println("Please set the ELASTIC_APM_SERVER_TOKEN environment variable")
		os.Exit(1)
	}
	forceBuildLambdasFlag := false
	if os.Getenv("FORCE_REBUILD_LOCAL_SAM_LAMBDAS") == "true" {
		forceBuildLambdasFlag = true
	}

	buildExtensionBinaries()
	if !folderExists(filepath.Join("sam-java", "agent")) {
		log.Println("Java agent not found ! Collecting archive from Github...")
		retrieveJavaAgent("sam-java")
	}
	changeJavaAgentPermissions("sam-java")

	if os.Getenv("PARALLEL_EXECUTION") == "true" {
		var waitGroup sync.WaitGroup
		waitGroup.Add(3)

		cNode := make(chan bool)
		go runTestAsync("sam-node", "SamTestingNode", forceBuildLambdasFlag, &waitGroup, cNode)
		cPython := make(chan bool)
		go runTestAsync("sam-python", "SamTestingPython", forceBuildLambdasFlag, &waitGroup, cPython)
		cJava := make(chan bool)
		go runTestAsync("sam-java", "SamTestingJava", forceBuildLambdasFlag, &waitGroup, cJava)
		assert.True(t, <-cNode)
		assert.True(t, <-cPython)
		assert.True(t, <-cJava)
		waitGroup.Wait()

	} else {
		assert.True(t, runTest("sam-node", "SamTestingNode", forceBuildLambdasFlag))
		assert.True(t, runTest("sam-python", "SamTestingPython", forceBuildLambdasFlag))
		assert.True(t, runTest("sam-java", "SamTestingJava", forceBuildLambdasFlag))
	}

}

func buildExtensionBinaries() {
	runCommandInDir("make", []string{}, "..", os.Getenv("DEBUG_OUTPUT") == "true")
}

func runTest(path string, serviceName string, buildFlag bool) bool {
	log.Printf("Starting to test %s", serviceName)

	if !folderExists(filepath.Join(path, ".aws-sam")) || buildFlag {
		log.Printf("Building the Lambda function %s", serviceName)
		runCommandInDir("sam", []string{"build"}, path, os.Getenv("DEBUG_OUTPUT") == "true")
	}

	log.Printf("Invoking the Lambda function %s", serviceName)
	uuidWithHyphen := uuid.New().String()
	runCommandInDir("sam", []string{"local", "invoke", "--parameter-overrides",
		fmt.Sprintf("ParameterKey=ApmServerURL,ParameterValue=%s", os.Getenv("ELASTIC_APM_SERVER_URL")),
		fmt.Sprintf("ParameterKey=ApmSecretToken,ParameterValue=%s", os.Getenv("ELASTIC_APM_SERVER_TOKEN")),
		fmt.Sprintf("ParameterKey=TestUUID,ParameterValue=%s", uuidWithHyphen)},
		path, os.Getenv("DEBUG_OUTPUT") == "true")
	log.Printf("%s execution complete", serviceName)

	log.Printf("Querying Elasticsearch for transaction %s bound to %s...", uuidWithHyphen, serviceName)
	res := queryElasticsearch(serviceName, uuidWithHyphen)
	if res {
		log.Printf("SUCCESS : Transaction %s successfully sent to Elasticsearch by %s", uuidWithHyphen, serviceName)
		return true
	}
	log.Printf("FAILURE : Transaction %s bound to %s not found in Elasticsearch", uuidWithHyphen, serviceName)
	return false
}

func runTestAsync(path string, serviceName string, buildFlag bool, wg *sync.WaitGroup, c chan bool) {
	defer wg.Done()
	out := runTest(path, serviceName, buildFlag)
	c <- out
}

func retrieveJavaAgent(samJavaPath string) {

	agentFolderPath := filepath.Join(samJavaPath, "agent")
	agentArchivePath := filepath.Join(samJavaPath, "agent.zip")

	// Download archive
	out, err := os.Create(agentArchivePath)
	processError(err)
	defer out.Close()
	resp, err := http.Get("https://github.com/elastic/apm-agent-java/releases/download/v1.28.4/elastic-apm-java-aws-lambda-layer-1.28.4.zip")
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

func queryElasticsearch(serviceName string, transactionName string) bool {

	cfg := elasticsearch.Config{
		CloudID: os.Getenv("ELASTIC_CLOUD_ID"),
		APIKey:  os.Getenv("ELASTIC_CLOUD_API_KEY"),
	}

	es, err := elasticsearch.NewClient(cfg)

	body := fmt.Sprintf(`{
  				"query": {
    				"bool": {
      					"must": [
        					{
          						"match": {
            						"service.name": "%s"
          						}
        					},
        					{
          						"match": {
            						"transaction.name": "%s"
          						}
        					}
      					]
    				}
  				}
			}`, serviceName, transactionName)

	res, err := es.Count(
		es.Count.WithIndex("apm-*-transaction-*"),
		es.Count.WithBody(strings.NewReader(body)),
		es.Count.WithPretty(),
	)
	processError(err)
	defer res.Body.Close()

	var r map[string]interface{}
	err = json.NewDecoder(res.Body).Decode(&r)
	processError(err)
	hitsNum := int(r["count"].(float64))
	if hitsNum > 0 {
		return true
	}
	return false
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
	if os.IsNotExist(err) {
		return false
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
