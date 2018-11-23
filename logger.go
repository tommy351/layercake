package main

import (
	"github.com/ansel1/merry"
	"github.com/sirupsen/logrus"
	"github.com/x-cray/logrus-prefixed-formatter"
)

var logger = logrus.New()

func initLogger() (err error) {
	formatter := &prefixed.TextFormatter{}

	if globalOptions.Debug {
		merry.SetVerboseDefault(true)
		logger.SetLevel(logrus.DebugLevel)
		logger.SetReportCaller(true)
		formatter.TimestampFormat = "15:04:05"
		formatter.FullTimestamp = true
	} else {
		formatter.DisableTimestamp = true
	}

	logger.SetFormatter(formatter)

	return
}
