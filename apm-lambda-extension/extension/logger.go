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
	"fmt"
	"go.elastic.co/ecszap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"strings"
)

type Level uint32

const (
	TraceLevel Level = iota
	DebugLevel
	InfoLevel
	WarnLevel
	ErrorLevel
	CriticalLevel
	OffLevel
)

func (lvl Level) String() string {
	switch lvl {
	case TraceLevel:
		return "trace"
	case DebugLevel:
		return "debug"
	case InfoLevel:
		return "info"
	case WarnLevel:
		return "warn"
	case ErrorLevel:
		return "error"
	case CriticalLevel:
		return "critical"
	case OffLevel:
		return "off"
	}
	return "unknown"
}

type LevelLogger struct {
	logger *zap.Logger
	level  Level
	config zap.Config
}

var Log LevelLogger

func init() {
	Log.config = zap.NewProductionConfig()
	Log.config.EncoderConfig = ecszap.ECSCompatibleEncoderConfig(Log.config.EncoderConfig)
	Log.config.EncoderConfig.LevelKey = zapcore.OmitKey
	// This needs to be set so that later calls to log.Debugf work - our level system is decoupled from zap's system
	Log.config.Level.SetLevel(zap.DebugLevel)
	Log.logger, _ = Log.config.Build(ecszap.WrapCoreOption(), zap.AddCaller(), zap.AddCallerSkip(2))
}

// ParseLogLevel parses s as a logrus log level.
func ParseLogLevel(s string) (Level, error) {
	switch strings.ToLower(s) {
	case "trace":
		return TraceLevel, nil
	case "debug":
		return DebugLevel, nil
	case "info":
		return InfoLevel, nil
	case "warn", "warning":
		// "warn" exists for backwards compatibility;
		// "warning" is the canonical level name.
		return WarnLevel, nil
	case "error":
		return ErrorLevel, nil
	case "critical":
		return CriticalLevel, nil
	case "off":
		return OffLevel, nil
	}
	return OffLevel, fmt.Errorf("invalid log level string %s", s)
}

func (l *LevelLogger) SetOutputPaths(paths []string) {
	Log.config.OutputPaths = paths
	Log.logger, _ = Log.config.Build(ecszap.WrapCoreOption(), zap.AddCaller(), zap.AddCallerSkip(2))
}

func (l *LevelLogger) SetLogLevel(level Level) error {
	switch level {
	case TraceLevel:
		l.level = TraceLevel
		return nil
	case DebugLevel:
		l.level = DebugLevel
		return nil
	case InfoLevel:
		l.level = InfoLevel
		return nil
	case WarnLevel:
		l.level = WarnLevel
		return nil
	case ErrorLevel:
		l.level = ErrorLevel
		return nil
	case CriticalLevel:
		l.level = CriticalLevel
		return nil
	case OffLevel:
		l.level = OffLevel
		return nil
	}
	l.level = InfoLevel
	return fmt.Errorf("invalid log level string %s, defaulting to Info level", level.String())
}

func (l *LevelLogger) Trace(msg string) {
	l.log(TraceLevel, msg)
}
func (l *LevelLogger) Tracef(format string, args ...interface{}) {
	l.log(TraceLevel, fmt.Sprintf(format, args...))
}

func (l *LevelLogger) Debug(msg string) {
	l.log(DebugLevel, msg)
}
func (l *LevelLogger) Debugf(format string, args ...interface{}) {
	l.log(DebugLevel, fmt.Sprintf(format, args...))
}

func (l *LevelLogger) Info(msg string) {
	l.log(InfoLevel, msg)
}
func (l *LevelLogger) Infof(format string, args ...interface{}) {
	l.log(InfoLevel, fmt.Sprintf(format, args...))
}

func (l *LevelLogger) Warn(msg string) {
	l.log(WarnLevel, msg)
}
func (l *LevelLogger) Warnf(format string, args ...interface{}) {
	l.log(WarnLevel, fmt.Sprintf(format, args...))
}

func (l *LevelLogger) Error(msg string) {
	l.log(ErrorLevel, msg)
}
func (l *LevelLogger) Errorf(format string, args ...interface{}) {
	l.log(ErrorLevel, fmt.Sprintf(format, args...))
}

func (l *LevelLogger) Critical(msg string) {
	l.log(CriticalLevel, msg)
}
func (l *LevelLogger) Criticalf(format string, args ...interface{}) {
	l.log(CriticalLevel, fmt.Sprintf(format, args...))
}

func (l *LevelLogger) log(level Level, msg string) {
	if level < l.level {
		return
	}

	// apm <-> zap level mapping
	switch level {
	case TraceLevel, DebugLevel:
		l.logger.Debug(msg, zap.String("log.level", level.String()))
	case InfoLevel:
		l.logger.Info(msg, zap.String("log.level", level.String()))
	case WarnLevel:
		l.logger.Warn(msg, zap.String("log.level", level.String()))
	case ErrorLevel:
		l.logger.Error(msg, zap.String("log.level", level.String()))
	case CriticalLevel:
		l.logger.Fatal(msg, zap.String("log.level", level.String()))
	}
}
