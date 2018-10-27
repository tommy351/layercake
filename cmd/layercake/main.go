package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/jessevdk/go-flags"
	"github.com/tommy351/layercake"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v2"
)

type Args struct {
	LogLevel zapcore.Level `long:"log-level" description:"Log level" default:"info"`
	Config   string        `long:"config" description:"Config path" default:"layercake.yml"`
}

func main() {
	var args Args
	parser := flags.NewParser(&args, flags.Default)

	if _, err := parser.Parse(); err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			os.Exit(1)
		}
	}

	logger := newLogger(args.LogLevel)
	config, err := readConfig(args.Config)

	if err != nil {
		logger.Fatal("Failed to read the config", zap.Error(err))
	}

	gracefulRun(func(ctx context.Context) {
		builder := &layercake.Builder{
			Context:   ctx,
			Config:    config,
			ImageName: getWorkingDirName() + "_%s",
			Logger:    logger,
		}

		if err := builder.Build(); err != nil {
			logger.Fatal("Build failed", zap.Error(err))
		}
	})
}

func newLogger(level zapcore.Level) *zap.Logger {
	conf := zap.NewDevelopmentConfig()
	conf.Level = zap.NewAtomicLevelAt(level)
	logger, err := conf.Build()

	if err != nil {
		panic(err)
	}

	return logger
}

func readConfig(path string) (*layercake.Config, error) {
	file, err := os.Open(path)

	if err != nil {
		return nil, err
	}

	defer file.Close()

	var config layercake.Config

	if err := yaml.NewDecoder(file).Decode(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

func gracefulRun(fn func(ctx context.Context)) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-ch
		cancel()
	}()

	fn(ctx)
}

func getWorkingDirName() string {
	if wd, err := os.Getwd(); err == nil {
		return filepath.Base(wd)
	}

	return "layercake"
}
