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

package logger

import (
	"fmt"
	"strings"

	"go.elastic.co/ecszap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New returns a logger.
func New(opts ...Option) (*zap.SugaredLogger, error) {
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
