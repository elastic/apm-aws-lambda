package logger_test

import (
	"elastic/apm-lambda-extension/logger"
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

	assert.Regexp(t, `{"log.level":"info","@timestamp":".*","log.origin":{"file.name":"logger/logger_test.go","file.line":.*},"message":"logger-test-info","ecs.version":"1.6.0"}`, string(tempFileContents))
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

	assert.Regexp(t, `{"log.level":"debug","@timestamp":".*","log.origin":{"file.name":"logger/logger_test.go","file.line":.*},"message":"logger-test-trace","ecs.version":"1.6.0"}`, string(tempFileContents))
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
