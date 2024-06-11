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
	DisableLineBreak bool
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
	var id ID
	var hasId bool
	if ctx != nil {
		id, hasId = IDFromContext(ctx)
	}
	if hasId {
		activeDuration := FormatDuration(time.Since(id.CreatedAt))
		if !f.DisableColors {
			var color aurora.Color
			color = aurora.Color(uint8(id.ID))
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
			message = F.ToString("[", aurora.Colorize(id.ID, color).String(), " ", activeDuration, "] ", message)
		} else {
			message = F.ToString("[", id.ID, " ", activeDuration, "] ", message)
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
	if f.DisableLineBreak {
		if message[len(message)-1] == '\n' {
			message = message[:len(message)-1]
		}
	} else {
		if message[len(message)-1] != '\n' {
			message += "\n"
		}
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
	var id ID
	var hasId bool
	if ctx != nil {
		id, hasId = IDFromContext(ctx)
	}
	if hasId {
		activeDuration := FormatDuration(time.Since(id.CreatedAt))
		if !f.DisableColors {
			var color aurora.Color
			color = aurora.Color(uint8(id.ID))
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
			message = F.ToString("[", aurora.Colorize(id.ID, color).String(), " ", activeDuration, "] ", message)
		} else {
			message = F.ToString("[", id.ID, " ", activeDuration, "] ", message)
		}
		messageSimple = F.ToString("[", id.ID, " ", activeDuration, "] ", messageSimple)

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

func FormatDuration(duration time.Duration) string {
	if duration < time.Second {
		return F.ToString(duration.Milliseconds(), "ms")
	} else if duration < time.Minute {
		return F.ToString(int64(duration.Seconds()), ".", int64(duration.Seconds()*100)%100, "s")
	} else {
		return F.ToString(int64(duration.Minutes()), "m", int64(duration.Seconds())%60, "s")
	}
}
