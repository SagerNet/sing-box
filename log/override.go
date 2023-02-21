package log

import (
	"context"
)

type overrideLevelKey struct{}

func ContextWithOverrideLevel(ctx context.Context, level Level) context.Context {
	return context.WithValue(ctx, (*overrideLevelKey)(nil), level)
}

func OverrideLevelFromContext(origin Level, ctx context.Context) Level {
	level, loaded := ctx.Value((*overrideLevelKey)(nil)).(Level)
	if !loaded || origin > level {
		return origin
	}
	return level
}
