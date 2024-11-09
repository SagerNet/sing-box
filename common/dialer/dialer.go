package dialer

import (
	"context"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-dns"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

func New(ctx context.Context, options option.DialerOptions) (N.Dialer, error) {
	router := service.FromContext[adapter.Router](ctx)
	if options.IsWireGuardListener {
		return NewDefault(router, options)
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
		outboundManager := service.FromContext[adapter.OutboundManager](ctx)
		if outboundManager == nil {
			return nil, E.New("missing outbound manager")
		}
		dialer = NewDetour(outboundManager, options.Detour)
	}
	if router == nil {
		return NewDefault(router, options)
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
