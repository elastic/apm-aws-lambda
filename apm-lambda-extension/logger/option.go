package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type option func(*zap.Config)

func WithLevel(level zapcore.Level) option {
	return func(c *zap.Config) {
		c.Level.SetLevel(level)
	}
}

func WithEncoderConfig(encoderConfig zapcore.EncoderConfig) option {
	return func(c *zap.Config) {
		c.EncoderConfig = encoderConfig
	}
}

func WithOutputPaths(path string) option {
	return func(c *zap.Config) {
		c.OutputPaths = append(c.OutputPaths, path)
	}
}
