package log

import (
	"context"
	"math/rand"

	"github.com/sagernet/sing/common/random"
)

func init() {
	random.InitializeSeed()
}

var idType = (*idContext)(nil)

type idContext struct {
	context.Context
	id uint32
}

func (c *idContext) Value(key any) any {
	if key == idType {
		return c
	}
	return c.Context.Value(key)
}

func ContextWithID(ctx context.Context) context.Context {
	if ctx.Value(idType) != nil {
		return ctx
	}
	return &idContext{ctx, rand.Uint32()}
}
