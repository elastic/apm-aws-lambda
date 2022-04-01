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
	"github.com/sirupsen/logrus"
	"go.elastic.co/ecslogrus"
	"io/ioutil"
	"os"
	"strings"
)

var Log *logrus.Entry

func init() {
	newLogger := logrus.New()
	newLogger.SetFormatter(&ecslogrus.Formatter{})
	newLogger.SetLevel(logrus.TraceLevel)
	newLoggerWithFields := newLogger.WithFields(logrus.Fields{"event.dataset": "apm-lambda-extension"})
	Log = newLoggerWithFields
}

// ParseLogLevel parses s as a logrus log level.
func ParseLogLevel(s string) (logrus.Level, error) {
	// Set default output. Required to start logging back if the prior level was "off"
	logrus.SetOutput(os.Stderr)
	switch strings.ToLower(s) {
	case "trace":
		return logrus.TraceLevel, nil
	case "debug":
		return logrus.DebugLevel, nil
	case "info":
		return logrus.InfoLevel, nil
	case "warn", "warning":
		// "warn" exists for backwards compatibility;
		// "warning" is the canonical level name.
		return logrus.WarnLevel, nil
	case "error":
		return logrus.ErrorLevel, nil
	case "fatal", "critical":
		return logrus.FatalLevel, nil
	case "off":
		logrus.SetOutput(ioutil.Discard)
		return logrus.PanicLevel, nil
	}
	return logrus.InfoLevel, fmt.Errorf("invalid log level string %q", s)
}
