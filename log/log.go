package log

import (
	"context"

	"github.com/sagernet/sing-box/option"
)

type Logger interface {
	Start() error
	Close() error
	Trace(args ...interface{})
	Debug(args ...interface{})
	Info(args ...interface{})
	Print(args ...interface{})
	Warn(args ...interface{})
	Warning(args ...interface{})
	Error(args ...interface{})
	Fatal(args ...interface{})
	Panic(args ...interface{})
	WithContext(ctx context.Context) Logger
	WithPrefix(prefix string) Logger
}

func NewLogger(options option.LogOption) (Logger, error) {
	if options.Disabled {
		return NewNopLogger(), nil
	}
	return NewLogrusLogger(options)
}
