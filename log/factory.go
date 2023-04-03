package log

import (
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/common/observable"
)

type (
	Logger        logger.Logger
	ContextLogger logger.ContextLogger
)

type Factory interface {
	Level() Level
	SetLevel(level Level)
	Logger() ContextLogger
	NewLogger(tag string) ContextLogger
	Close() error
}

type ObservableFactory interface {
	Factory
	observable.Observable[Entry]
}

type Entry struct {
	Level   Level
	Message string
}
