package extension

import (
	"github.com/sirupsen/logrus"
	"go.elastic.co/ecslogrus"
)

var Log *logrus.Logger

func InitLogger() {
	if Log == nil {
		newLogger := logrus.New()
		newLogger.SetFormatter(&ecslogrus.Formatter{})
		newLogger.SetLevel(logrus.TraceLevel)
		Log = newLogger
	}
}
