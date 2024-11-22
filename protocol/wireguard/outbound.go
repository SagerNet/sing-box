package wireguard

import (
	"context"
	"net"
	"net/netip"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental/deprecated"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/wireguard"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register[option.LegacyWireGuardOutboundOptions](registry, C.TypeWireGuard, NewOutbound)
}

var (
	_ adapter.Endpoint                = (*Endpoint)(nil)
	_ adapter.InterfaceUpdateListener = (*Endpoint)(nil)
)

type Outbound struct {
	outbound.Adapter
	ctx            context.Context
	router         adapter.Router
	logger         logger.ContextLogger
	localAddresses []netip.Prefix
	endpoint       *wireguard.Endpoint
}

func NewOutbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.LegacyWireGuardOutboundOptions) (adapter.Outbound, error) {
	deprecated.Report(ctx, deprecated.OptionWireGuardOutbound)
	if options.GSO {
		deprecated.Report(ctx, deprecated.OptionWireGuardGSO)
	}
	outbound := &Outbound{
		Adapter:        outbound.NewAdapterWithDialerOptions(C.TypeWireGuard, tag, []string{N.NetworkTCP, N.NetworkUDP}, options.DialerOptions),
		ctx:            ctx,
		router:         router,
		logger:         logger,
		localAddresses: options.LocalAddress,
	}
	if options.Detour == "" {
		options.IsWireGuardListener = true
	} else if options.GSO {
		return nil, E.New("gso is conflict with detour")
	}
	outboundDialer, err := dialer.New(ctx, options.DialerOptions)
	if err != nil {
		return nil, err
	}
	wgEndpoint, err := wireguard.NewEndpoint(wireguard.EndpointOptions{
		Context: ctx,
		Logger:  logger,
		System:  options.SystemInterface,
		Dialer:  outboundDialer,
		CreateDialer: func(interfaceName string) N.Dialer {
			return common.Must1(dialer.NewDefault(service.FromContext[adapter.NetworkManager](ctx), option.DialerOptions{
				BindInterface: interfaceName,
			}))
		},
		Name:       options.InterfaceName,
		MTU:        options.MTU,
		Address:    options.LocalAddress,
		PrivateKey: options.PrivateKey,
		ResolvePeer: func(domain string) (netip.Addr, error) {
			endpointAddresses, lookupErr := router.Lookup(ctx, domain, dns.DomainStrategy(options.DomainStrategy))
			if lookupErr != nil {
				return netip.Addr{}, lookupErr
			}
			return endpointAddresses[0], nil
		},
		Peers: common.Map(options.Peers, func(it option.LegacyWireGuardPeer) wireguard.PeerOptions {
			return wireguard.PeerOptions{
				Endpoint:     it.ServerOptions.Build(),
				PublicKey:    it.PublicKey,
				PreSharedKey: it.PreSharedKey,
				AllowedIPs:   it.AllowedIPs,
				// PersistentKeepaliveInterval: time.Duration(it.PersistentKeepaliveInterval),
				Reserved: it.Reserved,
			}
		}),
		Workers: options.Workers,
	})
	if err != nil {
		return nil, err
	}
	outbound.endpoint = wgEndpoint
	return outbound, nil
}

func (o *Outbound) Start(stage adapter.StartStage) error {
	switch stage {
	case adapter.StartStateStart:
		return o.endpoint.Start(false)
	case adapter.StartStatePostStart:
		return o.endpoint.Start(true)
	}
	return nil
}

func (o *Outbound) Close() error {
	return o.endpoint.Close()
}

func (o *Outbound) InterfaceUpdated() {
	o.endpoint.BindUpdate()
	return
}

func (o *Outbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	switch network {
	case N.NetworkTCP:
		o.logger.InfoContext(ctx, "outbound connection to ", destination)
	case N.NetworkUDP:
		o.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	}
	if destination.IsFqdn() {
		destinationAddresses, err := o.router.LookupDefault(ctx, destination.Fqdn)
		if err != nil {
			return nil, err
		}
		return N.DialSerial(ctx, o.endpoint, network, destination, destinationAddresses)
	} else if !destination.Addr.IsValid() {
		return nil, E.New("invalid destination: ", destination)
	}
	return o.endpoint.DialContext(ctx, network, destination)
}

func (o *Outbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	o.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	if destination.IsFqdn() {
		destinationAddresses, err := o.router.LookupDefault(ctx, destination.Fqdn)
		if err != nil {
			return nil, err
		}
		packetConn, _, err := N.ListenSerial(ctx, o.endpoint, destination, destinationAddresses)
		if err != nil {
			return nil, err
		}
		return packetConn, err
	}
	return o.endpoint.ListenPacket(ctx, destination)
}
