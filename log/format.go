package log

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/logrusorgru/aurora"
)

type Formatter struct {
	BaseTime         time.Time
	DisableColors    bool
	DisableTimestamp bool
	FullTimestamp    bool
	TimestampFormat  string
	DisableLineBreak bool
}

var (
	levelLabels        [LevelTrace + 1]string
	coloredLevelLabels [LevelTrace + 1]string
)

func init() {
	for level := LevelPanic; level <= LevelTrace; level++ {
		label := strings.ToUpper(FormatLevel(level))
		levelLabels[level] = label
		switch level {
		case LevelDebug, LevelTrace:
			coloredLevelLabels[level] = aurora.White(label).String()
		case LevelInfo:
			coloredLevelLabels[level] = aurora.Cyan(label).String()
		case LevelWarn:
			coloredLevelLabels[level] = aurora.Yellow(label).String()
		case LevelError, LevelFatal, LevelPanic:
			coloredLevelLabels[level] = aurora.Red(label).String()
		}
	}
}

func (f Formatter) Format(ctx context.Context, level Level, tag string, message string, timestamp time.Time) string {
	var id ID
	var hasId bool
	if ctx != nil {
		id, hasId = IDFromContext(ctx)
	}
	var builder strings.Builder
	builder.Grow(len(tag) + len(message) + 64)
	f.writePrefix(&builder, level, timestamp)
	if hasId {
		f.writeIdPrefix(&builder, id)
	}
	if tag != "" {
		builder.WriteString(tag)
		builder.WriteString(": ")
	}
	if f.DisableLineBreak {
		builder.WriteString(strings.TrimSuffix(message, "\n"))
	} else {
		builder.WriteString(message)
		if !strings.HasSuffix(message, "\n") {
			builder.WriteByte('\n')
		}
	}
	return builder.String()
}

func (f Formatter) FormatSimple(ctx context.Context, tag string, message string) string {
	var id ID
	var hasId bool
	if ctx != nil {
		id, hasId = IDFromContext(ctx)
	}
	if !hasId && tag == "" {
		return message
	}
	var builder strings.Builder
	builder.Grow(len(tag) + len(message) + 32)
	if hasId {
		builder.WriteByte('[')
		writeUint(&builder, uint64(id.ID))
		builder.WriteByte(' ')
		writeDuration(&builder, time.Since(id.CreatedAt))
		builder.WriteString("] ")
	}
	if tag != "" {
		builder.WriteString(tag)
		builder.WriteString(": ")
	}
	builder.WriteString(message)
	return builder.String()
}

func (f Formatter) writePrefix(builder *strings.Builder, level Level, timestamp time.Time) {
	var levelString string
	if int(level) >= len(levelLabels) {
		levelString = "UNKNOWN"
	} else if f.DisableColors {
		levelString = levelLabels[level]
	} else {
		levelString = coloredLevelLabels[level]
	}
	switch {
	case f.DisableTimestamp:
		builder.WriteString(levelString)
		builder.WriteByte(' ')
	case f.FullTimestamp:
		var timeBuffer [64]byte
		builder.Write(timestamp.AppendFormat(timeBuffer[:0], f.TimestampFormat))
		builder.WriteByte(' ')
		builder.WriteString(levelString)
		builder.WriteByte(' ')
	default:
		builder.WriteString(levelString)
		builder.WriteByte('[')
		seconds := strconv.AppendInt(make([]byte, 0, 20), int64(timestamp.Sub(f.BaseTime)/time.Second), 10)
		for pad := 4 - len(seconds); pad > 0; pad-- {
			builder.WriteByte('0')
		}
		builder.Write(seconds)
		builder.WriteString("] ")
	}
}

func (f Formatter) writeIdPrefix(builder *strings.Builder, id ID) {
	builder.WriteByte('[')
	if f.DisableColors {
		writeUint(builder, uint64(id.ID))
	} else {
		builder.WriteString("\x1b[38;5;")
		writeUint(builder, uint64(colorForID(id.ID)))
		builder.WriteByte('m')
		writeUint(builder, uint64(id.ID))
		builder.WriteString("\x1b[0m")
	}
	builder.WriteByte(' ')
	writeDuration(builder, time.Since(id.CreatedAt))
	builder.WriteString("] ")
}

func colorForID(value uint32) uint8 {
	color := uint8(value) % 215
	row := uint(color / 36)
	column := uint(color % 36)
	r := float32(row * 51)
	g := float32(column / 6 * 51)
	b := float32((column % 6) * 51)
	luma := 0.2126*r + 0.7152*g + 0.0722*b
	if luma < 60 {
		row = 5 - row
		column = 35 - column
		color = uint8(row*36 + column)
	}
	return color + 16
}

func writeUint(builder *strings.Builder, value uint64) {
	builder.Write(strconv.AppendUint(make([]byte, 0, 20), value, 10))
}

func writeInt(builder *strings.Builder, value int64) {
	builder.Write(strconv.AppendInt(make([]byte, 0, 20), value, 10))
}

func writeDuration(builder *strings.Builder, duration time.Duration) {
	if duration < time.Second {
		writeInt(builder, duration.Milliseconds())
		builder.WriteString("ms")
	} else if duration < time.Minute {
		writeInt(builder, int64(duration.Seconds()))
		builder.WriteByte('.')
		writeInt(builder, int64(duration.Seconds()*100)%100)
		builder.WriteByte('s')
	} else {
		writeInt(builder, int64(duration.Minutes()))
		builder.WriteByte('m')
		writeInt(builder, int64(duration.Seconds())%60)
		builder.WriteByte('s')
	}
}

func FormatDuration(duration time.Duration) string {
	var builder strings.Builder
	writeDuration(&builder, duration)
	return builder.String()
}
