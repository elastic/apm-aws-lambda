package logger

import (
	"fmt"
	"strings"

	"go.elastic.co/ecszap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New returns a logger.
func New(opts ...option) (*zap.SugaredLogger, error) {
	conf := zap.NewProductionConfig()

	for _, opt := range opts {
		opt(&conf)
	}

	logger, err := conf.Build(ecszap.WrapCoreOption(), zap.AddCaller())
	if err != nil {
		return nil, err
	}

	return logger.Sugar(), nil
}

// ParseLogLevel parses s as a logrus log level. If the level is off, the return flag is set to true.
func ParseLogLevel(s string) (zapcore.Level, error) {
	switch strings.ToLower(s) {
	case "trace":
		return zapcore.DebugLevel, nil
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn", "warning":
		// "warn" exists for backwards compatibility;
		// "warning" is the canonical level name.
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	case "critical":
		return zapcore.FatalLevel, nil
	case "off":
		return zapcore.FatalLevel + 1, nil
	}
	return zapcore.InfoLevel, fmt.Errorf("invalid log level string %s", s)
}
