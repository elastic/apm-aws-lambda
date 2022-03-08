package extension

import (
	"github.com/sirupsen/logrus"
	"go.elastic.co/ecslogrus"
)

var Log *logrus.Logger

func InitLogger() *logrus.Logger {
	newLogger := logrus.New()
	newLogger.SetFormatter(&ecslogrus.Formatter{})
	newLogger.SetLevel(logrus.TraceLevel)
	return newLogger
}
