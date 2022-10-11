package outbound

import (
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/balancer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

var (
	_ adapter.Outbound      = (*LeastLoad)(nil)
	_ adapter.OutboundGroup = (*LeastLoad)(nil)
)

// LeastLoad is a outbound group that picks outbound with least load
type LeastLoad struct {
	*Balancer

	options option.BalancerOutboundOptions
}

// NewLeastLoad creates a new LeastLoad outbound
func NewLeastLoad(router adapter.Router, logger log.ContextLogger, tag string, options option.BalancerOutboundOptions) (*LeastLoad, error) {
	return &LeastLoad{
		Balancer: NewBalancer(
			C.TypeLeastLoad, router, logger, tag,
			options.Outbounds, options.Fallback,
		),
		options: options,
	}, nil
}

// Start implements common.Starter
func (s *LeastLoad) Start() error {
	err := s.Balancer.initialize()
	if err != nil {
		return err
	}
	b, err := balancer.NewLeastLoad(s.router, s.logger, s.options)
	if err != nil {
		return err
	}
	return s.setBalancer(b)
}
