package log

import (
	"context"
	"os"

	"github.com/sagernet/sing/common/observable"
)

var _ ObservableFactory = (*nopFactory)(nil)

type nopFactory struct{}

func NewNOPFactory() ObservableFactory {
	return (*nopFactory)(nil)
}

func (f *nopFactory) Start() error {
	return nil
}

func (f *nopFactory) Close() error {
	return nil
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

func (f *nopFactory) Subscribe() (subscription observable.Subscription[Entry], done <-chan struct{}, err error) {
	return nil, nil, os.ErrInvalid
}

func (f *nopFactory) UnSubscribe(subscription observable.Subscription[Entry]) {
}
