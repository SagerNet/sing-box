package log

import (
	"context"
	"strconv"
	"strings"
	"time"

	F "github.com/sagernet/sing/common/format"

	"github.com/logrusorgru/aurora"
)

type Formatter struct {
	BaseTime         time.Time
	DisableColors    bool
	DisableTimestamp bool
	FullTimestamp    bool
	TimestampFormat  string
}

func (f Formatter) Format(ctx context.Context, level Level, tag string, message string, timestamp time.Time) string {
	levelString := strings.ToUpper(FormatLevel(level))
	if !f.DisableColors {
		switch level {
		case LevelDebug, LevelTrace:
			levelString = aurora.White(levelString).String()
		case LevelInfo:
			levelString = aurora.Cyan(levelString).String()
		case LevelWarn:
			levelString = aurora.Yellow(levelString).String()
		case LevelError, LevelFatal, LevelPanic:
			levelString = aurora.Red(levelString).String()
		}
	}
	if tag != "" {
		message = tag + ": " + message
	}
	var id uint32
	var hasId bool
	if ctx != nil {
		id, hasId = IDFromContext(ctx)
	}
	if hasId {
		if !f.DisableColors {
			var color aurora.Color
			color = aurora.Color(uint8(id))
			color %= 215
			row := uint(color / 36)
			column := uint(color % 36)

			var r, g, b float32
			r = float32(row * 51)
			g = float32(column / 6 * 51)
			b = float32((column % 6) * 51)
			luma := 0.2126*r + 0.7152*g + 0.0722*b
			if luma < 60 {
				row = 5 - row
				column = 35 - column
				color = aurora.Color(row*36 + column)
			}
			color += 16
			color = color << 16
			color |= 1 << 14
			message = F.ToString("[", aurora.Colorize(id, color).String(), "] ", message)
		} else {
			message = F.ToString("[", id, "] ", message)
		}
	}
	switch {
	case f.DisableTimestamp:
		message = levelString + " " + message
	case f.FullTimestamp:
		message = timestamp.Format(f.TimestampFormat) + " " + levelString + " " + message
	default:
		message = levelString + "[" + xd(int(timestamp.Sub(f.BaseTime)/time.Second), 4) + "] " + message
	}
	if message[len(message)-1] != '\n' {
		message += "\n"
	}
	return message
}

func (f Formatter) FormatWithSimple(ctx context.Context, level Level, tag string, message string, timestamp time.Time) (string, string) {
	levelString := strings.ToUpper(FormatLevel(level))
	if !f.DisableColors {
		switch level {
		case LevelDebug, LevelTrace:
			levelString = aurora.White(levelString).String()
		case LevelInfo:
			levelString = aurora.Cyan(levelString).String()
		case LevelWarn:
			levelString = aurora.Yellow(levelString).String()
		case LevelError, LevelFatal, LevelPanic:
			levelString = aurora.Red(levelString).String()
		}
	}
	if tag != "" {
		message = tag + ": " + message
	}
	messageSimple := message
	var id uint32
	var hasId bool
	if ctx != nil {
		id, hasId = IDFromContext(ctx)
	}
	if hasId {
		if !f.DisableColors {
			var color aurora.Color
			color = aurora.Color(uint8(id))
			color %= 215
			row := uint(color / 36)
			column := uint(color % 36)

			var r, g, b float32
			r = float32(row * 51)
			g = float32(column / 6 * 51)
			b = float32((column % 6) * 51)
			luma := 0.2126*r + 0.7152*g + 0.0722*b
			if luma < 60 {
				row = 5 - row
				column = 35 - column
				color = aurora.Color(row*36 + column)
			}
			color += 16
			color = color << 16
			color |= 1 << 14
			message = F.ToString("[", aurora.Colorize(id, color).String(), "] ", message)
		} else {
			message = F.ToString("[", id, "] ", message)
		}
		messageSimple = F.ToString("[", id, "] ", messageSimple)

	}
	switch {
	case f.DisableTimestamp:
		message = levelString + " " + message
	case f.FullTimestamp:
		message = timestamp.Format(f.TimestampFormat) + " " + levelString + " " + message
	default:
		message = levelString + "[" + xd(int(timestamp.Sub(f.BaseTime)/time.Second), 4) + "] " + message
	}
	if message[len(message)-1] != '\n' {
		message += "\n"
	}
	return message, messageSimple
}

func xd(value int, x int) string {
	message := strconv.Itoa(value)
	for len(message) < x {
		message = "0" + message
	}
	return message
}
