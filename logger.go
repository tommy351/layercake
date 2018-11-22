package main

import (
	"time"

	"github.com/ansel1/merry"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger = zap.NewNop()

func initLogger() (err error) {
	loggerConfig := zap.Config{
		Encoding: "console",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "time",
			LevelKey:       "level",
			NameKey:        "name",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalColorLevelEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
		},
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}

	if globalOptions.Debug {
		merry.SetVerboseDefault(true)
		loggerConfig.Development = true
		loggerConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
		loggerConfig.EncoderConfig.EncodeTime = logShortTimeEncoder
		loggerConfig.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	} else {
		loggerConfig.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	if logger, err = loggerConfig.Build(); err != nil {
		return
	}

	return
}

func logShortTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("15:04:05"))
}
