package log

import (
	"context"
	"math/rand"
)

type idContext struct {
	context.Context
	id uint32
}

func ContextWithID(ctx context.Context) context.Context {
	return &idContext{ctx, rand.Uint32()}
}
