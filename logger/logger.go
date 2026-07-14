package logger

import (
	"context"
	"log"
)

const ContextKey = "logger"

func New(prefix string) *log.Logger {
	return log.New(log.Writer(), "["+prefix+"] ", log.Flags())
}

func FromContext(ctx context.Context) *log.Logger {
	l, ok := ctx.Value(ContextKey).(*log.Logger)
	if !ok || l == nil {
		return log.Default()
	}
	return l
}

func Printf(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).Printf(msg, args...)
}
