package log

import "context"

var _ Logger = (*nopLogger)(nil)

type nopLogger struct{}

func NewNopLogger() Logger {
	return (*nopLogger)(nil)
}

func (l *nopLogger) Start() error {
	return nil
}

func (l *nopLogger) Close() error {
	return nil
}

func (l *nopLogger) Trace(args ...interface{}) {
}

func (l *nopLogger) Debug(args ...interface{}) {
}

func (l *nopLogger) Info(args ...interface{}) {
}

func (l *nopLogger) Print(args ...interface{}) {
}

func (l *nopLogger) Warn(args ...interface{}) {
}

func (l *nopLogger) Warning(args ...interface{}) {
}

func (l *nopLogger) Error(args ...interface{}) {
}

func (l *nopLogger) Fatal(args ...interface{}) {
}

func (l *nopLogger) Panic(args ...interface{}) {
}

func (l *nopLogger) WithContext(ctx context.Context) Logger {
	return l
}

func (l *nopLogger) WithPrefix(prefix string) Logger {
	return l
}
