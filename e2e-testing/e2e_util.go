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

package e2eTesting

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/elastic/apm-aws-lambda/apmproxy"

	"go.uber.org/zap"
)

// GetEnvVarValueOrSetDefault retrieves the environment variable envVarName.
// If the desired variable is not defined, defaultVal is returned.
func GetEnvVarValueOrSetDefault(envVarName string, defaultVal string) string {
	val := os.Getenv(envVarName)
	if val == "" {
		return defaultVal
	}
	return val
}

// RunCommandInDir runs a shell command with a given set of args in a specified folder.
// The stderr and stdout can be enabled or disabled.
func RunCommandInDir(l *zap.SugaredLogger, command string, args []string, dir string) {
	e := exec.Command(command, args...)
	e.Dir = dir
	stdout, _ := e.StdoutPipe()
	stderr, _ := e.StderrPipe()
	if err := e.Start(); err != nil {
		l.Errorf("Could not retrieve run %s : %v", command, err)
	}
	scannerOut := bufio.NewScanner(stdout)
	for scannerOut.Scan() {
		m := scannerOut.Text()
		l.Debugf(m)
	}
	scannerErr := bufio.NewScanner(stderr)
	for scannerErr.Scan() {
		m := scannerErr.Text()
		l.Debugf(m)
	}
	if err := e.Wait(); err != nil {
		l.Errorf("Could not wait for the execution of %s : %v", command, err)
	}

}

// FolderExists returns true if the specified folder exists, and false else.
func FolderExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ProcessError is a shorthand function to handle fatal errors, the idiomatic Go way.
// This should only be used for showstopping errors.
func ProcessError(l *zap.SugaredLogger, err error) {
	if err != nil {
		l.Panic(err.Error())
	}
}

// Unzip is a utility function that unzips a specified zip archive to a specified destination.
func Unzip(l *zap.SugaredLogger, archivePath string, destinationFolderPath string) {

	openedArchive, err := zip.OpenReader(archivePath)
	ProcessError(l, err)
	defer openedArchive.Close()

	// Permissions setup
	err = os.MkdirAll(destinationFolderPath, 0755)
	if err != nil {
		l.Errorf("Could not create folders required to unzip, %v", err)
	}

	// Closure required, so that Close() calls do not pile up when unzipping archives with a lot of files
	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			if err = rc.Close(); err != nil {
				panic(err)
			}
		}()

		path := filepath.Join(destinationFolderPath, f.Name)

		// Check for ZipSlip (Directory traversal)
		if !strings.HasPrefix(path, filepath.Clean(destinationFolderPath)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", path)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, f.Mode()); err != nil {
				l.Errorf("Could not unzip folder : %v", err)
			}
		} else {
			if err = os.MkdirAll(filepath.Dir(path), f.Mode()); err != nil {
				l.Errorf("Could not unzip file : %v", err)
			}
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			ProcessError(l, err)
			defer f.Close()
			_, err = io.Copy(f, rc)
			ProcessError(l, err)
		}
		return nil
	}

	for _, f := range openedArchive.File {
		err := extractAndWriteFile(f)
		ProcessError(l, err)
	}
}

// IsStringInSlice is a utility function that checks if a slice of strings contains a specific string.
func IsStringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// GetDecompressedBytesFromRequest takes a HTTP request in argument and return the raw (decompressed) bytes of the body.
// The byte array can then be converted into a string for debugging / testing purposes.
func GetDecompressedBytesFromRequest(req *http.Request) ([]byte, error) {
	var rawBytes []byte
	if req.Body != nil {
		rawBytes, _ = io.ReadAll(req.Body)
	}
	return apmproxy.GetUncompressedBytes(rawBytes, req.Header.Get("Content-Encoding"))
}

// GetFreePort is a function that queries the kernel and obtains an unused port.
func GetFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
