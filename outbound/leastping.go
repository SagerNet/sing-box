package outbound

import (
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/balancer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

var (
	_ adapter.Outbound      = (*LeastPing)(nil)
	_ adapter.OutboundGroup = (*LeastPing)(nil)
)

// LeastPing is a outbound group that picks outbound with least load
type LeastPing struct {
	*Balancer

	options option.LeastPingOutboundOptions
}

// NewLeastPing creates a new LeastPing outbound
func NewLeastPing(router adapter.Router, logger log.ContextLogger, tag string, options option.LeastPingOutboundOptions) (*LeastPing, error) {
	if len(options.Outbounds) == 0 {
		return nil, E.New("missing tags")
	}
	return &LeastPing{
		Balancer: NewBalancer(
			C.TypeLeastPing, router, logger, tag,
			options.Outbounds, options.Fallback,
		),
		options: options,
	}, nil
}

// Start implements common.Starter
func (s *LeastPing) Start() error {
	err := s.Balancer.initialize()
	if err != nil {
		return err
	}
	b, err := balancer.NewLeastPing(s.nodes, s.logger, s.options)
	if err != nil {
		return err
	}
	return s.setBalancer(b)
}
