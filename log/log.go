package log

import (
	"context"

	"github.com/logrusorgru/aurora"
	"github.com/sagernet/sing/common"
	F "github.com/sagernet/sing/common/format"
	"github.com/sirupsen/logrus"
)

type Logger interface {
	logrus.FieldLogger
	WithContext(ctx context.Context) *logrus.Entry
}

type Hook struct{}

func (h *Hook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *Hook) Fire(entry *logrus.Entry) error {
	if prefix, loaded := entry.Data["prefix"]; loaded {
		prefixStr := prefix.(string)
		delete(entry.Data, "prefix")
		entry.Message = prefixStr + entry.Message
	}
	if idCtx, loaded := common.Cast[*idContext](entry.Context); loaded {
		var color aurora.Color
		color = aurora.Color(uint8(idCtx.id))
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
		entry.Message = F.ToString("[", aurora.Colorize(idCtx.id, color).String(), "] ", entry.Message)
	}
	return nil
}
