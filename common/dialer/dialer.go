package dialer

import (
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing/common"
	N "github.com/sagernet/sing/common/network"
)

func New(router adapter.Router, options option.DialerOptions) N.Dialer {
	if options.Detour == "" {
		return NewDefault(router, options)
	} else {
		return NewDetour(router, options.Detour)
	}
}

func NewOutbound(router adapter.Router, options option.OutboundDialerOptions) N.Dialer {
	dialer := New(router, options.DialerOptions)
	domainStrategy := dns.DomainStrategy(options.DomainStrategy)
	if domainStrategy != dns.DomainStrategyAsIS || options.Detour == "" {
		dialer = NewResolveDialer(router, dialer, domainStrategy, time.Duration(options.FallbackDelay))
	}
	if options.OverrideOptions.IsValid() {
		dialer = NewOverride(dialer, common.PtrValueOrDefault(options.OverrideOptions))
	}
	return dialer
}
