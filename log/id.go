package log

import (
	"context"
	"math/rand"

	"github.com/sagernet/sing/common/random"
)

func init() {
	random.InitializeSeed()
}

type idKey struct{}

func ContextWithNewID(ctx context.Context) context.Context {
	return context.WithValue(ctx, (*idKey)(nil), rand.Uint32())
}

func IDFromContext(ctx context.Context) (uint32, bool) {
	id, loaded := ctx.Value((*idKey)(nil)).(uint32)
	return id, loaded
}
