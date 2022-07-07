package dialer

import (
	"github.com/sagernet/sing/common"
	N "github.com/sagernet/sing/common/network"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

func New(router adapter.Router, options option.DialerOptions) N.Dialer {
	domainStrategy := C.DomainStrategy(options.DomainStrategy)
	var dialer N.Dialer
	if options.Detour == "" {
		dialer = NewDefault(options)
		dialer = NewResolveDialer(router, dialer, domainStrategy)
	} else {
		dialer = NewDetour(router, options.Detour)
		if domainStrategy != C.DomainStrategyAsIS {
			dialer = NewResolveDialer(router, dialer, domainStrategy)
		}
	}
	if options.OverrideOptions.IsValid() {
		dialer = NewOverride(dialer, common.PtrValueOrDefault(options.OverrideOptions))
	}
	return dialer
}
