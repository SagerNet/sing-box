package log

import (
	"context"
	"os"

	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"

	"github.com/sagernet/sing-box/option"

	"github.com/sirupsen/logrus"
)

var _ Logger = (*logrusLogger)(nil)

type logrusLogger struct {
	abstractLogrusLogger
	outputPath string
	output     *os.File
}

type abstractLogrusLogger interface {
	logrus.Ext1FieldLogger
	WithContext(ctx context.Context) *logrus.Entry
}

func NewLogrusLogger(options option.LogOption) (*logrusLogger, error) {
	logger := logrus.New()
	logger.SetLevel(logrus.TraceLevel)
	logger.SetFormatter(&LogrusTextFormatter{
		DisableColors:    options.DisableColor || options.Output != "",
		DisableTimestamp: !options.Timestamp && options.Output != "",
		FullTimestamp:    options.Timestamp,
	})
	logger.AddHook(new(logrusHook))
	var err error
	if options.Level != "" {
		logger.Level, err = logrus.ParseLevel(options.Level)
		if err != nil {
			return nil, err
		}
	}
	return &logrusLogger{logger, options.Output, nil}, nil
}

func (l *logrusLogger) Start() error {
	if l.outputPath != "" {
		output, err := os.OpenFile(l.outputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return E.Cause(err, "open log output")
		}
		l.abstractLogrusLogger.(*logrus.Logger).SetOutput(output)
	}
	return nil
}

func (l *logrusLogger) Close() error {
	return common.Close(common.PtrOrNil(l.output))
}

func (l *logrusLogger) WithContext(ctx context.Context) Logger {
	return &logrusLogger{abstractLogrusLogger: l.abstractLogrusLogger.WithContext(ctx)}
}

func (l *logrusLogger) WithPrefix(prefix string) Logger {
	if entry, isEntry := l.abstractLogrusLogger.(*logrus.Entry); isEntry {
		loadedPrefix := entry.Data["prefix"]
		if loadedPrefix != "" {
			prefix = F.ToString(loadedPrefix, prefix)
		}
	}
	return &logrusLogger{abstractLogrusLogger: l.WithField("prefix", prefix)}
}
