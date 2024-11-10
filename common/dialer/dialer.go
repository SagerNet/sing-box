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
	networkManager := service.FromContext[adapter.NetworkManager](ctx)
	if options.IsWireGuardListener {
		return NewDefault(networkManager, options)
	}
	var (
		dialer N.Dialer
		err    error
	)
	if options.Detour == "" {
		dialer, err = NewDefault(networkManager, options)
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
	if networkManager == nil {
		return NewDefault(networkManager, options)
	}
	if options.Detour == "" {
		router := service.FromContext[adapter.Router](ctx)
		if router != nil {
			dialer = NewResolveDialer(
				router,
				dialer,
				options.Detour == "" && !options.TCPFastOpen,
				dns.DomainStrategy(options.DomainStrategy),
				time.Duration(options.FallbackDelay))
		}
	}
	return dialer, nil
}
