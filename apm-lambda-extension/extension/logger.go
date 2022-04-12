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

type LevelLogger struct {
	*zap.SugaredLogger
	zap.Config
}

var Log LevelLogger

func init() {
	// Init Log level to info
	atom := zap.NewAtomicLevel()
	// Set ECS logging config
	Log.Config = zap.NewProductionConfig()
	Log.Config.Level = atom
	Log.Config.EncoderConfig = ecszap.NewDefaultEncoderConfig().ToZapCoreEncoderConfig()
	// Create ECS logger
	logger, _ := Log.Config.Build(ecszap.WrapCoreOption(), zap.AddCaller())
	Log.SugaredLogger = logger.Sugar()
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
		return zapcore.PanicLevel, nil
	case "off":
		return zapcore.FatalLevel, nil
	}
	return zapcore.InfoLevel, fmt.Errorf("invalid log level string %s", s)
}

func SetLogOutputPaths(paths []string) {
	Log.Config.OutputPaths = paths
	logger, err := Log.Config.Build(ecszap.WrapCoreOption(), zap.AddCaller())
	if err != nil {
		Log.Errorf("Could not set log path : %v", err)
	}
	Log.SugaredLogger = logger.Sugar()
}
