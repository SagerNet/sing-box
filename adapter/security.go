package adapter

import (
	"context"

	"github.com/sagernet/sing/service"
)

type SecurityPolicy interface {
	CheckFeature(feature string) error
}

func CheckSecurityFeature(ctx context.Context, feature string) error {
	policy := service.FromContext[SecurityPolicy](ctx)
	if policy == nil {
		return nil
	}
	return policy.CheckFeature(feature)
}
