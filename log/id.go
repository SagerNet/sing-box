package log

import (
	"context"
	"math/rand"
	"time"

	"github.com/sagernet/sing/common/random"
)

func init() {
	random.InitializeSeed()
}

type idKey struct{}

type ID struct {
	ID        uint32
	CreatedAt time.Time
}

func ContextWithNewID(ctx context.Context) context.Context {
	return ContextWithID(ctx, ID{
		ID:        rand.Uint32(),
		CreatedAt: time.Now(),
	})
}

func ContextWithID(ctx context.Context, id ID) context.Context {
	return context.WithValue(ctx, (*idKey)(nil), id)
}

func IDFromContext(ctx context.Context) (ID, bool) {
	id, loaded := ctx.Value((*idKey)(nil)).(ID)
	return id, loaded
}
