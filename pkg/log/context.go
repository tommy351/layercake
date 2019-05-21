package log

import (
	"context"

	"go.uber.org/zap"
)

// nolint: gochecknoglobals
var (
	contextKey = &struct{}{}
	nullLogger = zap.NewNop()
)

func NewContext(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, contextKey, logger)
}

func FromContext(ctx context.Context) *zap.Logger {
	logger := ctx.Value(contextKey)

	if logger != nil {
		if logger, ok := logger.(*zap.Logger); ok {
			return logger
		}
	}

	return nullLogger
}
