package log

import (
	"context"

	"github.com/sagernet/sing/common/observable"
)

type Factory interface {
	Level() Level
	SetLevel(level Level)
	Logger() ContextLogger
	NewLogger(tag string) ContextLogger
}

type ObservableFactory interface {
	Factory
	observable.Observable[Entry]
}

type Entry struct {
	Level   Level
	Message string
}

type Logger interface {
	Trace(args ...any)
	Debug(args ...any)
	Info(args ...any)
	Warn(args ...any)
	Error(args ...any)
	Fatal(args ...any)
	Panic(args ...any)
}

type ContextLogger interface {
	Logger
	TraceContext(ctx context.Context, args ...any)
	DebugContext(ctx context.Context, args ...any)
	InfoContext(ctx context.Context, args ...any)
	WarnContext(ctx context.Context, args ...any)
	ErrorContext(ctx context.Context, args ...any)
	FatalContext(ctx context.Context, args ...any)
	PanicContext(ctx context.Context, args ...any)
}
