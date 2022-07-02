package log

import (
	"context"
	"os"

	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sirupsen/logrus"
)

var _ Logger = (*logrusLogger)(nil)

type logrusLogger struct {
	abstractLogrusLogger
	output *os.File
}

type abstractLogrusLogger interface {
	logrus.Ext1FieldLogger
	WithContext(ctx context.Context) *logrus.Entry
}

func NewLogrusLogger(options option.LogOption) (*logrusLogger, error) {
	logger := logrus.New()
	logger.SetLevel(logrus.TraceLevel)
	logger.Formatter.(*logrus.TextFormatter).ForceColors = true
	logger.AddHook(new(logrusHook))
	var output *os.File
	var err error
	if options.Level != "" {
		logger.Level, err = logrus.ParseLevel(options.Level)
		if err != nil {
			return nil, err
		}
	}
	if options.Output != "" {
		output, err = os.OpenFile(options.Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, E.Extend(err, "open log output")
		}
		logger.SetOutput(output)
	}
	return &logrusLogger{logger, output}, nil
}

func (l *logrusLogger) WithContext(ctx context.Context) Logger {
	return &logrusLogger{l.abstractLogrusLogger.WithContext(ctx), nil}
}

func (l *logrusLogger) WithPrefix(prefix string) Logger {
	if entry, isEntry := l.abstractLogrusLogger.(*logrus.Entry); isEntry {
		loadedPrefix := entry.Data["prefix"]
		if loadedPrefix != "" {
			prefix = F.ToString(loadedPrefix, prefix)
		}
	}
	return &logrusLogger{l.WithField("prefix", prefix), nil}
}

func (l *logrusLogger) Close() error {
	return common.Close(common.PtrOrNil(l.output))
}
