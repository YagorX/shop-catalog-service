package contracts

import (
	"context"
	"log/slog"
)

type LogLevelController interface {
	SetLevel(level string) error
	Level() slog.Level
}

type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

type ReadinessChecker interface {
	Check(ctx context.Context) error
}
