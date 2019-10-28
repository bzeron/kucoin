package kucoin

import (
	"github.com/sirupsen/logrus"
	"os"
)

var (
	Logger = logrus.New()
)

func init() {
	Logger.SetFormatter(&logrus.TextFormatter{})
	Logger.SetOutput(os.Stdout)
	Logger.SetLevel(logrus.TraceLevel)
}
