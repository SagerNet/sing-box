package vision

import (
	"context"
	"net"
)

type Hook func(net.Conn)

type hookKey struct{}

func WithHook(ctx context.Context, hook Hook) context.Context {
	if hook == nil {
		return ctx
	}
	return context.WithValue(ctx, hookKey{}, hook)
}

func HookFromContext(ctx context.Context) (Hook, bool) {
	if ctx == nil {
		return nil, false
	}
	hook, ok := ctx.Value(hookKey{}).(Hook)
	return hook, ok
}
