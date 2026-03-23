package tls

import (
	"strings"

	"github.com/sagernet/sing/common/logger"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ACMELogWriter struct {
	Logger logger.Logger
}

func (w *ACMELogWriter) Write(p []byte) (n int, err error) {
	logLine := strings.ReplaceAll(string(p), "	", ": ")
	switch {
	case strings.HasPrefix(logLine, "error: "):
		w.Logger.Error(logLine[7:])
	case strings.HasPrefix(logLine, "warn: "):
		w.Logger.Warn(logLine[6:])
	case strings.HasPrefix(logLine, "info: "):
		w.Logger.Info(logLine[6:])
	case strings.HasPrefix(logLine, "debug: "):
		w.Logger.Debug(logLine[7:])
	default:
		w.Logger.Debug(logLine)
	}
	return len(p), nil
}

func (w *ACMELogWriter) Sync() error {
	return nil
}

func ACMEEncoderConfig() zapcore.EncoderConfig {
	config := zap.NewProductionEncoderConfig()
	config.TimeKey = zapcore.OmitKey
	return config
}
