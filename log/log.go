package log

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/service/filemanager"
)

type factoryWithFile struct {
	Factory
	file *os.File
}

func (f *factoryWithFile) Close() error {
	return common.Close(
		f.Factory,
		common.PtrOrNil(f.file),
	)
}

type observableFactoryWithFile struct {
	ObservableFactory
	file *os.File
}

func (f *observableFactoryWithFile) Close() error {
	return common.Close(
		f.ObservableFactory,
		common.PtrOrNil(f.file),
	)
}

type Options struct {
	Context        context.Context
	Options        option.LogOptions
	Observable     bool
	DefaultWriter  io.Writer
	BaseTime       time.Time
	PlatformWriter io.Writer
}

func New(options Options) (Factory, error) {
	logOptions := options.Options

	if logOptions.Disabled {
		return NewNOPFactory(), nil
	}

	var logFile *os.File
	var logWriter io.Writer

	switch logOptions.Output {
	case "":
		logWriter = options.DefaultWriter
		if logWriter == nil {
			logWriter = os.Stderr
		}
	case "stderr":
		logWriter = os.Stderr
	case "stdout":
		logWriter = os.Stdout
	default:
		var err error
		logFile, err = filemanager.OpenFile(options.Context, logOptions.Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, err
		}
		logWriter = logFile
	}
	logFormatter := Formatter{
		BaseTime:         options.BaseTime,
		DisableColors:    logOptions.DisableColor || logFile != nil,
		DisableTimestamp: !logOptions.Timestamp && logFile != nil,
		FullTimestamp:    logOptions.Timestamp,
		TimestampFormat:  "-0700 2006-01-02 15:04:05",
	}
	var factory Factory
	if options.Observable {
		factory = NewObservableFactory(logFormatter, logWriter, options.PlatformWriter)
	} else {
		factory = NewFactory(logFormatter, logWriter, options.PlatformWriter)
	}
	if logOptions.Level != "" {
		logLevel, err := ParseLevel(logOptions.Level)
		if err != nil {
			return nil, E.Cause(err, "parse log level")
		}
		factory.SetLevel(logLevel)
	} else {
		factory.SetLevel(LevelTrace)
	}
	if logFile != nil {
		if options.Observable {
			factory = &observableFactoryWithFile{
				ObservableFactory: factory.(ObservableFactory),
				file:              logFile,
			}
		} else {
			factory = &factoryWithFile{
				Factory: factory,
				file:    logFile,
			}
		}
	}
	return factory, nil
}
