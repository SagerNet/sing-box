package log

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	red    = 31
	yellow = 33
	blue   = 36
	gray   = 37
)

var baseTimestamp time.Time

func init() {
	baseTimestamp = time.Now()
}

type LogrusTextFormatter struct {
	DisableColors    bool
	DisableTimestamp bool
	FullTimestamp    bool
	TimestampFormat  string
}

func (f *LogrusTextFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var b *bytes.Buffer
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}
	timestampFormat := f.TimestampFormat
	if timestampFormat == "" {
		timestampFormat = "-0700 2006-01-02 15:04:05"
	}
	f.print(b, entry, timestampFormat)
	b.WriteByte('\n')
	return b.Bytes(), nil
}

func (f *LogrusTextFormatter) print(b *bytes.Buffer, entry *logrus.Entry, timestampFormat string) {
	var levelColor int
	switch entry.Level {
	case logrus.DebugLevel, logrus.TraceLevel:
		levelColor = gray
	case logrus.WarnLevel:
		levelColor = yellow
	case logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel:
		levelColor = red
	case logrus.InfoLevel:
		levelColor = blue
	default:
		levelColor = blue
	}

	levelText := strings.ToUpper(entry.Level.String())
	if !f.DisableColors {
		switch {
		case f.DisableTimestamp:
			fmt.Fprintf(b, "\x1b[%dm%s\x1b[0m %-44s", levelColor, levelText, entry.Message)
		case !f.FullTimestamp:
			fmt.Fprintf(b, "\x1b[%dm%s\x1b[0m[%04d] %-44s", levelColor, levelText, int(entry.Time.Sub(baseTimestamp)/time.Second), entry.Message)
		default:
			fmt.Fprintf(b, "%s \x1b[%dm%s\x1b[0m %-44s", entry.Time.Format(timestampFormat), levelColor, levelText, entry.Message)
		}
	} else {
		switch {
		case f.DisableTimestamp:
			fmt.Fprintf(b, "%s %-44s", levelText, entry.Message)
		case !f.FullTimestamp:
			fmt.Fprintf(b, "%s[%04d] %-44s", levelText, int(entry.Time.Sub(baseTimestamp)/time.Second), entry.Message)
		default:
			fmt.Fprintf(b, "[%s] %s %-44s", entry.Time.Format(timestampFormat), levelText, entry.Message)
		}
	}
}
