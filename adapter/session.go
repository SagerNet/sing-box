package adapter

import "context"

type keepSessionKey struct{}

func ContextWithKeepSession(ctx context.Context) context.Context {
	return context.WithValue(ctx, (*keepSessionKey)(nil), true)
}

func KeepSessionFromContext(ctx context.Context) bool {
	keep, _ := ctx.Value((*keepSessionKey)(nil)).(bool)
	return keep
}
