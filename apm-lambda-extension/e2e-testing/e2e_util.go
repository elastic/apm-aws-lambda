package e2e_testing

import (
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"elastic/apm-lambda-extension/extension"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func GetEnvVarValueOrSetDefault(envVarName string, defaultVal string) string {
	val := os.Getenv(envVarName)
	if val == "" {
		return defaultVal
	}
	return val
}

func RunCommandInDir(command string, args []string, dir string) {
	e := exec.Command(command, args...)
	extension.Log.Tracef(e.String())
	e.Dir = dir
	stdout, _ := e.StdoutPipe()
	stderr, _ := e.StderrPipe()
	e.Start()
	scannerOut := bufio.NewScanner(stdout)
	for scannerOut.Scan() {
		m := scannerOut.Text()
		extension.Log.Tracef(m)
	}
	scannerErr := bufio.NewScanner(stderr)
	for scannerErr.Scan() {
		m := scannerErr.Text()
		extension.Log.Tracef(m)
	}
	e.Wait()

}

func FolderExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	return false
}

func ProcessError(err error) {
	if err != nil {
		extension.Log.Error(err)
	}
}

func Unzip(archivePath string, destinationFolderPath string) {

	openedArchive, err := zip.OpenReader(archivePath)
	ProcessError(err)
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
			ProcessError(err)
			defer f.Close()
			_, err = io.Copy(f, rc)
			ProcessError(err)
		}
		return nil
	}

	for _, f := range openedArchive.File {
		err := extractAndWriteFile(f)
		ProcessError(err)
	}
}

func IsStringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func GetDecompressedBytesFromRequest(req *http.Request) ([]byte, error) {
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
