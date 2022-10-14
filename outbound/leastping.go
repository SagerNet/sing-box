package outbound

import (
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/balancer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

var (
	_ adapter.Outbound      = (*LeastPing)(nil)
	_ adapter.OutboundGroup = (*LeastPing)(nil)
	_ adapter.Service       = (*LeastPing)(nil)
)

// LeastPing is a outbound group that picks outbound with least load
type LeastPing struct {
	*Balancer

	options option.BalancerOutboundOptions
}

// NewLeastPing creates a new LeastPing outbound
func NewLeastPing(router adapter.Router, logger log.ContextLogger, tag string, options option.BalancerOutboundOptions) (*LeastPing, error) {
	return &LeastPing{
		Balancer: NewBalancer(
			C.TypeLeastPing, router, logger, tag,
			options.Outbounds, options.Fallback,
		),
		options: options,
	}, nil
}

// Start implements adapter.Service
func (s *LeastPing) Start() error {
	b, err := balancer.NewLeastPing(s.router, s.logger, s.options)
	if err != nil {
		return err
	}
	s.Balancer.Balancer = b
	return s.Balancer.Start()
}
