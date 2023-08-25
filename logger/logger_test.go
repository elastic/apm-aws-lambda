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

package logger_test

import (
	"github.com/elastic/apm-aws-lambda/logger"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.elastic.co/ecszap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestDefaultLogger(t *testing.T) {
	tempFile, err := os.CreateTemp(t.TempDir(), "tempFileLoggerTest-")
	require.NoError(t, err)
	defer tempFile.Close()

	l, err := logger.New(
		logger.WithEncoderConfig(ecszap.NewDefaultEncoderConfig().ToZapCoreEncoderConfig()),
		logger.WithOutputPaths(tempFile.Name()),
	)
	require.NoError(t, err)

	l.Infof("%s", "logger-test-info")
	l.Debugf("%s", "logger-test-debug")

	tempFileContents, err := os.ReadFile(tempFile.Name())
	require.NoError(t, err)

	assert.Regexp(t, `{"log.level":"info","@timestamp":".*","log.origin":{"function":"github.com/elastic/apm-aws-lambda/logger_test.TestDefaultLogger","file.name":"logger/logger_test.go","file.line":.*},"message":"logger-test-info","ecs.version":"1.6.0"}`, string(tempFileContents))
}

func TestLoggerParseLogLevel(t *testing.T) {
	testCases := []struct {
		level         string
		expectedLevel zapcore.Level
		expectedErr   bool
	}{
		{
			level:         "TRacE",
			expectedLevel: zapcore.DebugLevel,
		},
		{
			level:         "dEbuG",
			expectedLevel: zapcore.DebugLevel,
		},
		{
			level:         "InFo",
			expectedLevel: zapcore.InfoLevel,
		},
		{
			level:         "WaRning",
			expectedLevel: zapcore.WarnLevel,
		},
		{
			level:         "eRRor",
			expectedLevel: zapcore.ErrorLevel,
		},
		{
			level:         "CriTicaL",
			expectedLevel: zapcore.FatalLevel,
		},
		{
			level:         "OFF",
			expectedLevel: zapcore.FatalLevel + 1,
		},
		{
			level:       "Inva@Lid3",
			expectedErr: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.level, func(t *testing.T) {
			l, err := logger.ParseLogLevel(tc.level)
			if tc.expectedErr {
				require.Error(t, err)
			} else {
				require.Equal(t, tc.expectedLevel, l)
			}
		})
	}
}

func TestLoggerSetLogLevel(t *testing.T) {
	tempFile, err := os.CreateTemp(t.TempDir(), "tempFileLoggerTest-")
	require.NoError(t, err)
	defer tempFile.Close()

	l, err := logger.New(
		logger.WithEncoderConfig(ecszap.NewDefaultEncoderConfig().ToZapCoreEncoderConfig()),
		logger.WithOutputPaths(tempFile.Name()),
		logger.WithLevel(zap.DebugLevel),
	)
	require.NoError(t, err)

	l.Debugf("%s", "logger-test-trace")
	tempFileContents, err := os.ReadFile(tempFile.Name())
	require.NoError(t, err)

	assert.Regexp(t, `{"log.level":"debug","@timestamp":".*","log.origin":{"function":"github.com/elastic/apm-aws-lambda/logger_test.TestLoggerSetLogLevel","file.name":"logger/logger_test.go","file.line":.*},"message":"logger-test-trace","ecs.version":"1.6.0"}`, string(tempFileContents))
}

func TestLoggerSetOffLevel(t *testing.T) {
	tempFile, err := os.CreateTemp(t.TempDir(), "tempFileLoggerTest-")
	require.NoError(t, err)
	defer tempFile.Close()

	l, err := logger.New(
		logger.WithOutputPaths(tempFile.Name()),
		logger.WithLevel(zap.FatalLevel+1),
	)
	require.NoError(t, err)

	l.Errorf("%s", "logger-test-trace")
	tempFileContents, err := os.ReadFile(tempFile.Name())
	require.NoError(t, err)
	assert.Equal(t, "", string(tempFileContents))
}
