package experimental

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

var dynamicManagerConstructor DynamicManagerConstructor

type DynamicManagerConstructor func(ctx context.Context, logger log.ContextLogger, options option.DynamicAPIOptions) (adapter.DynamicManager, error)

func RegisterDynamicManagerConstructor(constructor DynamicManagerConstructor) {
	dynamicManagerConstructor = constructor
}

func NewDynamicManager(ctx context.Context, logger log.ContextLogger, options option.DynamicAPIOptions) (adapter.DynamicManager, error) {
	return dynamicManagerConstructor(ctx, logger, options)
}
