package extension

import (
	"github.com/sirupsen/logrus"
	"go.elastic.co/ecslogrus"
)

var Log *logrus.Entry

func init() {

	if Log == nil {
		newLogger := logrus.New()
		newLogger.SetFormatter(&ecslogrus.Formatter{})
		newLogger.SetLevel(logrus.TraceLevel)
		newLoggerWithFields := newLogger.WithFields(logrus.Fields{"event.dataset": "apm-lambda-extension"})
		Log = newLoggerWithFields
	}
}
