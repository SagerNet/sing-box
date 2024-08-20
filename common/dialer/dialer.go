package dialer

import (
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-dns"
	N "github.com/sagernet/sing/common/network"
)

func New(router adapter.Router, options option.DialerOptions) (N.Dialer, error) {
	if options.IsWireGuardListener {
		return NewDefault(router, options)
	}
	if router == nil {
		return NewDefault(nil, options)
	}
	var (
		dialer N.Dialer
		err    error
	)
	if options.Detour == "" {
		dialer, err = NewDefault(router, options)
		if err != nil {
			return nil, err
		}
	} else {
		dialer = NewDetour(router, options.Detour)
	}
	if options.Detour == "" {
		dialer = NewResolveDialer(
			router,
			dialer,
			options.Detour == "" && !options.TCPFastOpen,
			dns.DomainStrategy(options.DomainStrategy),
			time.Duration(options.FallbackDelay))
	}
	return dialer, nil
}
