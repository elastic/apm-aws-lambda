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

package extension

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"testing"
)

func init() {
	os.Unsetenv("ELASTIC_APM_LOG_LEVEL")
}

func TestInitLogger(t *testing.T) {
	assert.NotNil(t, Log)
}

func TestDefaultLogger(t *testing.T) {
	tempFile, err := ioutil.TempFile(os.TempDir(), "tempFileLoggerTest-")
	require.NoError(t, err)
	defer tempFile.Close()

	Log.SetOutputPaths([]string{tempFile.Name()})
	defer Log.SetOutputPaths([]string{"stderr"})

	Log.Infof("%s", "logger-test-info")
	Log.Debugf("%s", "logger-test-debug")
	tempFileContents, err := ioutil.ReadFile(tempFile.Name())
	require.NoError(t, err)
	assert.Regexp(t, `{"@timestamp":".*","log.origin":{"file.name":"extension/logger_test.go","file.line":.*},"message":"logger-test-info","log.level":"info","ecs.version":"1.6.0"}`, string(tempFileContents))
}

func TestLoggerParseLogLevel(t *testing.T) {
	trace, _ := ParseLogLevel("TRacE")
	debug, _ := ParseLogLevel("dEbuG")
	info, _ := ParseLogLevel("InFo")
	warn, _ := ParseLogLevel("WaRning")
	errorL, _ := ParseLogLevel("eRRor")
	critical, _ := ParseLogLevel("CriTicaL")
	off, _ := ParseLogLevel("OFF")
	invalid, err := ParseLogLevel("Inva@Lid3")
	assert.Equal(t, TraceLevel, trace)
	assert.Equal(t, DebugLevel, debug)
	assert.Equal(t, InfoLevel, info)
	assert.Equal(t, WarnLevel, warn)
	assert.Equal(t, ErrorLevel, errorL)
	assert.Equal(t, CriticalLevel, critical)
	assert.Equal(t, OffLevel, off)
	assert.Equal(t, OffLevel, invalid)
	assert.Error(t, err, "invalid log level string Inva@Lid3")
}

func TestLoggerSetLogLevel(t *testing.T) {
	tempFile, err := ioutil.TempFile(os.TempDir(), "tempFileLoggerTest-")
	require.NoError(t, err)
	defer tempFile.Close()

	err = Log.SetLogLevel(TraceLevel)
	require.NoError(t, err)

	defer func() {
		err := Log.SetLogLevel(InfoLevel)
		require.NoError(t, err)
	}()

	Log.SetOutputPaths([]string{tempFile.Name()})
	defer Log.SetOutputPaths([]string{"stderr"})

	Log.Tracef("%s", "logger-test-trace")
	tempFileContents, err := ioutil.ReadFile(tempFile.Name())
	require.NoError(t, err)
	assert.Regexp(t, `{"@timestamp":".*","log.origin":{"file.name":"extension/logger_test.go","file.line":.*},"message":"logger-test-trace","log.level":"trace","ecs.version":"1.6.0"}`, string(tempFileContents))
}

func TestLoggerSetInvalidLevel(t *testing.T) {
	err := Log.SetLogLevel(255)
	assert.Error(t, err, "invalid log level string unknown, defaulting to Info level")
	assert.Equal(t, Log.level, InfoLevel)
}

func TestLoggerSetOffLevel(t *testing.T) {
	tempFile, err := ioutil.TempFile(os.TempDir(), "tempFileLoggerTest-")
	require.NoError(t, err)
	defer tempFile.Close()

	err = Log.SetLogLevel(OffLevel)
	require.NoError(t, err)

	defer func() {
		err := Log.SetLogLevel(InfoLevel)
		require.NoError(t, err)
	}()

	Log.SetOutputPaths([]string{tempFile.Name()})
	defer Log.SetOutputPaths([]string{"stderr"})

	Log.Errorf("%s", "logger-test-error")
	Log.Infof("%s", "logger-test-info")
	Log.Debugf("%s", "logger-test-debug")
	tempFileContents, err := ioutil.ReadFile(tempFile.Name())
	require.NoError(t, err)
	assert.Regexp(t, "", string(tempFileContents))
}
