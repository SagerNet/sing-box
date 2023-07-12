package log

import (
	"context"
	"io"
	"os"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/observable"
)

var _ Factory = (*observableFactory)(nil)

type observableFactory struct {
	formatter         Formatter
	platformFormatter Formatter
	writer            io.Writer
	platformWriter    io.Writer
	level             Level
	subscriber        *observable.Subscriber[Entry]
	observer          *observable.Observer[Entry]
}

func NewObservableFactory(formatter Formatter, writer io.Writer, platformWriter io.Writer) ObservableFactory {
	factory := &observableFactory{
		formatter: formatter,
		platformFormatter: Formatter{
			BaseTime:         formatter.BaseTime,
			DisableColors:    C.IsDarwin || C.IsIos,
			DisableLineBreak: true,
		},
		writer:         writer,
		platformWriter: platformWriter,
		level:          LevelTrace,
		subscriber:     observable.NewSubscriber[Entry](128),
	}
	factory.observer = observable.NewObserver[Entry](factory.subscriber, 64)
	return factory
}

func (f *observableFactory) Level() Level {
	return f.level
}

func (f *observableFactory) SetLevel(level Level) {
	f.level = level
}

func (f *observableFactory) Logger() ContextLogger {
	return f.NewLogger("")
}

func (f *observableFactory) NewLogger(tag string) ContextLogger {
	return &observableLogger{f, tag}
}

func (f *observableFactory) Subscribe() (subscription observable.Subscription[Entry], done <-chan struct{}, err error) {
	return f.observer.Subscribe()
}

func (f *observableFactory) UnSubscribe(sub observable.Subscription[Entry]) {
	f.observer.UnSubscribe(sub)
}

func (f *observableFactory) Close() error {
	return common.Close(
		f.observer,
	)
}

var _ ContextLogger = (*observableLogger)(nil)

type observableLogger struct {
	*observableFactory
	tag string
}

func (l *observableLogger) Log(ctx context.Context, level Level, args []any) {
	level = OverrideLevelFromContext(level, ctx)
	if level > l.level {
		return
	}
	nowTime := time.Now()
	message, messageSimple := l.formatter.FormatWithSimple(ctx, level, l.tag, F.ToString(args...), nowTime)
	if level == LevelPanic {
		panic(message)
	}
	l.writer.Write([]byte(message))
	if level == LevelFatal {
		os.Exit(1)
	}
	l.subscriber.Emit(Entry{level, messageSimple})
	if l.platformWriter != nil {
		l.platformWriter.Write([]byte(l.platformFormatter.Format(ctx, level, l.tag, F.ToString(args...), nowTime)))
	}
}

func (l *observableLogger) Trace(args ...any) {
	l.TraceContext(context.Background(), args...)
}

func (l *observableLogger) Debug(args ...any) {
	l.DebugContext(context.Background(), args...)
}

func (l *observableLogger) Info(args ...any) {
	l.InfoContext(context.Background(), args...)
}

func (l *observableLogger) Warn(args ...any) {
	l.WarnContext(context.Background(), args...)
}

func (l *observableLogger) Error(args ...any) {
	l.ErrorContext(context.Background(), args...)
}

func (l *observableLogger) Fatal(args ...any) {
	l.FatalContext(context.Background(), args...)
}

func (l *observableLogger) Panic(args ...any) {
	l.PanicContext(context.Background(), args...)
}

func (l *observableLogger) TraceContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelTrace, args)
}

func (l *observableLogger) DebugContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelDebug, args)
}

func (l *observableLogger) InfoContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelInfo, args)
}

func (l *observableLogger) WarnContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelWarn, args)
}

func (l *observableLogger) ErrorContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelError, args)
}

func (l *observableLogger) FatalContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelFatal, args)
}

func (l *observableLogger) PanicContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelPanic, args)
}
