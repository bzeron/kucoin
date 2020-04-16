package kucoin

import (
	"os"

	"github.com/sirupsen/logrus"
)

var (
	Logger = logrus.New()
)

func init() {
	Logger.SetFormatter(&logrus.TextFormatter{})
	Logger.SetOutput(os.Stdout)
	Logger.SetLevel(logrus.TraceLevel)
}
