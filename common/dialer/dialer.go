package dialer

import (
	"github.com/sagernet/sing/common"
	N "github.com/sagernet/sing/common/network"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

func New(router adapter.Router, options option.DialerOptions) N.Dialer {
	if options.Detour == "" {
		return NewDefault(options)
	} else {
		return NewDetour(router, options.Detour)
	}
}

func NewOutbound(router adapter.Router, options option.OutboundDialerOptions) N.Dialer {
	dialer := New(router, options.DialerOptions)
	domainStrategy := C.DomainStrategy(options.DomainStrategy)
	if domainStrategy != C.DomainStrategyAsIS || options.Detour == "" && !C.CGO_ENABLED {
		dialer = NewResolveDialer(router, dialer, domainStrategy)
	}
	if options.OverrideOptions.IsValid() {
		dialer = NewOverride(dialer, common.PtrValueOrDefault(options.OverrideOptions))
	}
	return dialer
}
