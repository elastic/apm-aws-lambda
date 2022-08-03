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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

func init() {
	os.Unsetenv("ELASTIC_APM_LOG_LEVEL")
}

func TestInitLogger(t *testing.T) {
	assert.NotNil(t, Log)
}

func TestDefaultLogger(t *testing.T) {
	tempFile, err := os.CreateTemp(t.TempDir(), "tempFileLoggerTest-")
	require.NoError(t, err)
	defer tempFile.Close()

	SetLogOutputPaths([]string{tempFile.Name()})
	defer SetLogOutputPaths([]string{"stderr"})

	Log.Infof("%s", "logger-test-info")
	Log.Debugf("%s", "logger-test-debug")
	tempFileContents, err := os.ReadFile(tempFile.Name())
	require.NoError(t, err)
	assert.Regexp(t, `{"log.level":"info","@timestamp":".*","log.origin":{"file.name":"extension/logger_test.go","file.line":.*},"message":"logger-test-info","ecs.version":"1.6.0"}`, string(tempFileContents))
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
	assert.Equal(t, zapcore.DebugLevel, trace)
	assert.Equal(t, zapcore.DebugLevel, debug)
	assert.Equal(t, zapcore.InfoLevel, info)
	assert.Equal(t, zapcore.WarnLevel, warn)
	assert.Equal(t, zapcore.ErrorLevel, errorL)
	assert.Equal(t, zapcore.FatalLevel, critical)
	assert.Equal(t, zapcore.FatalLevel+1, off)
	assert.Equal(t, zapcore.InfoLevel, invalid)
	assert.Error(t, err, "invalid log level string Inva@Lid3")
}

func TestLoggerSetLogLevel(t *testing.T) {
	tempFile, err := os.CreateTemp(t.TempDir(), "tempFileLoggerTest-")
	require.NoError(t, err)
	defer tempFile.Close()

	Log.Level.SetLevel(zapcore.DebugLevel)

	defer Log.Level.SetLevel(zapcore.InfoLevel)

	SetLogOutputPaths([]string{tempFile.Name()})
	defer SetLogOutputPaths([]string{"stderr"})

	Log.Debugf("%s", "logger-test-trace")
	tempFileContents, err := os.ReadFile(tempFile.Name())
	require.NoError(t, err)
	assert.Regexp(t, `{"log.level":"debug","@timestamp":".*","log.origin":{"file.name":"extension/logger_test.go","file.line":.*},"message":"logger-test-trace","ecs.version":"1.6.0"}`, string(tempFileContents))
}

func TestLoggerSetOffLevel(t *testing.T) {
	tempFile, err := os.CreateTemp(t.TempDir(), "tempFileLoggerTest-")
	require.NoError(t, err)
	defer tempFile.Close()

	offLevel, _ := ParseLogLevel("off")
	Log.Level.SetLevel(offLevel)

	defer Log.Level.SetLevel(zapcore.InfoLevel)

	SetLogOutputPaths([]string{tempFile.Name()})
	defer SetLogOutputPaths([]string{"stderr"})

	Log.Errorf("%s", "logger-test-trace")
	tempFileContents, err := os.ReadFile(tempFile.Name())
	require.NoError(t, err)
	assert.Equal(t, "", string(tempFileContents))
}
