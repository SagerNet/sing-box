package service

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func New(ctx context.Context, router adapter.Router, logger log.ContextLogger, options option.Service) (adapter.BoxService, error) {
	if options.Type == "" {
		return nil, E.New("missing service type")
	}
	switch options.Type {
	case C.ServiceSubscription:
		return NewSubscription(router, logger, options)
	default:
		return nil, E.New("unknown service type: ", options.Type)
	}
}
