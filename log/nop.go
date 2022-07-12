package log

import "context"

var _ Factory = (*nopFactory)(nil)

type nopFactory struct{}

func NewNOPFactory() Factory {
	return (*nopFactory)(nil)
}

func (f *nopFactory) Level() Level {
	return LevelTrace
}

func (f *nopFactory) SetLevel(level Level) {
}

func (f *nopFactory) Logger() ContextLogger {
	return f
}

func (f *nopFactory) NewLogger(tag string) ContextLogger {
	return f
}

func (f *nopFactory) Trace(args ...any) {
}

func (f *nopFactory) Debug(args ...any) {
}

func (f *nopFactory) Info(args ...any) {
}

func (f *nopFactory) Warn(args ...any) {
}

func (f *nopFactory) Error(args ...any) {
}

func (f *nopFactory) Fatal(args ...any) {
}

func (f *nopFactory) Panic(args ...any) {
}

func (f *nopFactory) TraceContext(ctx context.Context, args ...any) {
}

func (f *nopFactory) DebugContext(ctx context.Context, args ...any) {
}

func (f *nopFactory) InfoContext(ctx context.Context, args ...any) {
}

func (f *nopFactory) WarnContext(ctx context.Context, args ...any) {
}

func (f *nopFactory) ErrorContext(ctx context.Context, args ...any) {
}

func (f *nopFactory) FatalContext(ctx context.Context, args ...any) {
}

func (f *nopFactory) PanicContext(ctx context.Context, args ...any) {
}
