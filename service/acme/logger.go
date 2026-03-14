//go:build with_acme

package acme

import (
	"strings"

	"github.com/sagernet/sing/common/logger"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type logWriter struct {
	logger logger.Logger
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	logLine := strings.ReplaceAll(string(p), "	", ": ")
	switch {
	case strings.HasPrefix(logLine, "error: "):
		w.logger.Error(logLine[7:])
	case strings.HasPrefix(logLine, "warn: "):
		w.logger.Warn(logLine[6:])
	case strings.HasPrefix(logLine, "info: "):
		w.logger.Info(logLine[6:])
	case strings.HasPrefix(logLine, "debug: "):
		w.logger.Debug(logLine[7:])
	default:
		w.logger.Debug(logLine)
	}
	return len(p), nil
}

func (w *logWriter) Sync() error {
	return nil
}

func encoderConfig() zapcore.EncoderConfig {
	config := zap.NewProductionEncoderConfig()
	config.TimeKey = zapcore.OmitKey
	return config
}
