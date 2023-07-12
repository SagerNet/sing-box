package log

import (
	"context"
	"io"
	"os"
	"time"

	C "github.com/sagernet/sing-box/constant"
	F "github.com/sagernet/sing/common/format"
)

var _ Factory = (*simpleFactory)(nil)

type simpleFactory struct {
	formatter         Formatter
	platformFormatter Formatter
	writer            io.Writer
	platformWriter    io.Writer
	level             Level
}

func NewFactory(formatter Formatter, writer io.Writer, platformWriter io.Writer) Factory {
	return &simpleFactory{
		formatter: formatter,
		platformFormatter: Formatter{
			BaseTime:         formatter.BaseTime,
			DisableColors:    C.IsDarwin || C.IsIos,
			DisableLineBreak: true,
		},
		writer:         writer,
		platformWriter: platformWriter,
		level:          LevelTrace,
	}
}

func (f *simpleFactory) Level() Level {
	return f.level
}

func (f *simpleFactory) SetLevel(level Level) {
	f.level = level
}

func (f *simpleFactory) Logger() ContextLogger {
	return f.NewLogger("")
}

func (f *simpleFactory) NewLogger(tag string) ContextLogger {
	return &simpleLogger{f, tag}
}

func (f *simpleFactory) Close() error {
	return nil
}

var _ ContextLogger = (*simpleLogger)(nil)

type simpleLogger struct {
	*simpleFactory
	tag string
}

func (l *simpleLogger) Log(ctx context.Context, level Level, args []any) {
	level = OverrideLevelFromContext(level, ctx)
	if level > l.level {
		return
	}
	nowTime := time.Now()
	message := l.formatter.Format(ctx, level, l.tag, F.ToString(args...), nowTime)
	if level == LevelPanic {
		panic(message)
	}
	l.writer.Write([]byte(message))
	if level == LevelFatal {
		os.Exit(1)
	}
	if l.platformWriter != nil {
		l.platformWriter.Write([]byte(l.platformFormatter.Format(ctx, level, l.tag, F.ToString(args...), nowTime)))
	}
}

func (l *simpleLogger) Trace(args ...any) {
	l.TraceContext(context.Background(), args...)
}

func (l *simpleLogger) Debug(args ...any) {
	l.DebugContext(context.Background(), args...)
}

func (l *simpleLogger) Info(args ...any) {
	l.InfoContext(context.Background(), args...)
}

func (l *simpleLogger) Warn(args ...any) {
	l.WarnContext(context.Background(), args...)
}

func (l *simpleLogger) Error(args ...any) {
	l.ErrorContext(context.Background(), args...)
}

func (l *simpleLogger) Fatal(args ...any) {
	l.FatalContext(context.Background(), args...)
}

func (l *simpleLogger) Panic(args ...any) {
	l.PanicContext(context.Background(), args...)
}

func (l *simpleLogger) TraceContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelTrace, args)
}

func (l *simpleLogger) DebugContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelDebug, args)
}

func (l *simpleLogger) InfoContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelInfo, args)
}

func (l *simpleLogger) WarnContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelWarn, args)
}

func (l *simpleLogger) ErrorContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelError, args)
}

func (l *simpleLogger) FatalContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelFatal, args)
}

func (l *simpleLogger) PanicContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelPanic, args)
}
