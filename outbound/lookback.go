package outbound

import "context"

type outboundTagKey struct{}

func ContextWithTag(ctx context.Context, outboundTag string) context.Context {
	return context.WithValue(ctx, outboundTagKey{}, outboundTag)
}

func TagFromContext(ctx context.Context) (string, bool) {
	value, loaded := ctx.Value(outboundTagKey{}).(string)
	return value, loaded
}
