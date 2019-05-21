package log

import (
	"time"

	"github.com/google/wire"
	"github.com/tommy351/layercake/pkg/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/xerrors"
)

// Set provides everything required for a logger.
// nolint: gochecknoglobals
var Set = wire.NewSet(NewEncoderConfig, NewLevel, NewLoggerConfig, NewLogger)

func ShortTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("15:04:05"))
}

func NewEncoderConfig() zapcore.EncoderConfig {
	conf := zap.NewDevelopmentEncoderConfig()
	conf.EncodeTime = ShortTimeEncoder
	conf.EncodeLevel = zapcore.CapitalColorLevelEncoder
	return conf
}

func NewLevel(conf *config.Config) (zap.AtomicLevel, error) {
	level := zap.NewAtomicLevel()

	if err := level.UnmarshalText([]byte(conf.Log.Level)); err != nil {
		return level, err
	}

	return level, nil
}

func NewLoggerConfig(encoderConfig zapcore.EncoderConfig, level zap.AtomicLevel) zap.Config {
	debug := level.Enabled(zap.DebugLevel)

	return zap.Config{
		Level:            level,
		Encoding:         "console",
		EncoderConfig:    encoderConfig,
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
		DisableCaller:    !debug,
		Development:      debug,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
	}
}

func NewLogger(conf zap.Config) (*zap.Logger, func(), error) {
	logger, err := conf.Build()

	if err != nil {
		return nil, nil, xerrors.Errorf("failed to create a logger: %w", err)
	}

	return logger, func() {
		_ = logger.Sync()
	}, err
}
