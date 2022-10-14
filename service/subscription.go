package service

import (
	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

var _ adapter.BoxService = (*Subscription)(nil)

// Subscription is a service that subscribes to remote servers for outbounds.
type Subscription struct {
	myServiceAdapter
}

func NewSubscription(router adapter.Router, logger log.ContextLogger, options option.Service) (*Subscription, error) {
	return &Subscription{
		myServiceAdapter{
			router:      router,
			serviceType: C.ServiceSubscription,
			logger:      logger,
			tag:         options.Tag,
		},
	}, nil
}

func (s *Subscription) Start() error {
	panic("implement me")
}

func (s *Subscription) Close() error {
	panic("implement me")
}
