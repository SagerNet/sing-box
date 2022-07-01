package log

import (
	"context"

	"github.com/sirupsen/logrus"
)

type Logger interface {
	logrus.FieldLogger
	WithContext(ctx context.Context) *logrus.Entry
}
