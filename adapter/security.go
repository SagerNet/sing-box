package adapter

import (
	"context"

	"github.com/sagernet/sing/service"
)

type SecurityPolicy interface {
	CheckFeature(ctx context.Context, feature string) error
}

func CheckSecurityFeature(ctx context.Context, feature string) error {
	policy := service.FromContext[SecurityPolicy](ctx)
	if policy == nil {
		return nil
	}
	return policy.CheckFeature(ctx, feature)
}
