package conntrack

import (
	"context"

	"github.com/sagernet/sing/service"
)

func ContextWithDefaultTracker(ctx context.Context, killerEnabled bool, memoryLimit uint64) context.Context {
	if service.FromContext[Tracker](ctx) != nil {
		return ctx
	}
	return service.ContextWith[Tracker](ctx, NewDefaultTracker(killerEnabled, memoryLimit))
}
