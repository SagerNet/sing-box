package log

import (
	"context"
	"os"
	"time"
)

var std ContextLogger

func init() {
	std = NewDefaultFactory(
		context.Background(),
		Formatter{BaseTime: time.Now()},
		os.Stderr,
		"",
		nil,
		false,
	).Logger()
}

func StdLogger() ContextLogger {
	return std
}

func SetStdLogger(logger ContextLogger) {
	std = logger
}

func Trace(args ...any) {
	std.Trace(args...)
}

func Debug(args ...any) {
	std.Debug(args...)
}

func Info(args ...any) {
	std.Info(args...)
}

func Warn(args ...any) {
	std.Warn(args...)
}

func Error(args ...any) {
	std.Error(args...)
}

func Fatal(args ...any) {
	std.Fatal(args...)
}

func Panic(args ...any) {
	std.Panic(args...)
}

func TraceContext(ctx context.Context, args ...any) {
	std.TraceContext(ctx, args...)
}

func DebugContext(ctx context.Context, args ...any) {
	std.DebugContext(ctx, args...)
}

func InfoContext(ctx context.Context, args ...any) {
	std.InfoContext(ctx, args...)
}

func WarnContext(ctx context.Context, args ...any) {
	std.WarnContext(ctx, args...)
}

func ErrorContext(ctx context.Context, args ...any) {
	std.ErrorContext(ctx, args...)
}

func FatalContext(ctx context.Context, args ...any) {
	std.FatalContext(ctx, args...)
}

func PanicContext(ctx context.Context, args ...any) {
	std.PanicContext(ctx, args...)
}
