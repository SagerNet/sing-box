//go:build with_tailscale

package tailscale

import (
	"context"
	"maps"
	"net"
	"net/netip"
	"slices"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/service/oomkiller"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/tailscale/types/key"
	"github.com/sagernet/wireguard-go/device"

	"go4.org/mem"
)

var (
	_ adapter.Inbound                 = (*TailcatInbound)(nil)
	_ adapter.InterfaceUpdateListener = (*TailcatInbound)(nil)
	_ adapter.Referrer                = (*TailcatInbound)(nil)
	_ tun.Handler                     = (*TailcatInbound)(nil)
)

func RegisterTailcatInbound(registry *inbound.Registry) {
	inbound.Register[option.TailcatInboundOptions](registry, C.TypeTailcat, NewTailcatInbound)
}

type TailcatInbound struct {
	inbound.Adapter
	ctx          context.Context
	router       adapter.Router
	logger       log.ContextLogger
	dnsRouter    adapter.DNSRouter
	dialer       N.Dialer
	queryOptions adapter.DNSQueryOptions
	detour       string
	privateKey   key.NodePrivate
	presharedKey device.NoisePresharedKey
	users        map[key.NodePublic]string
	derp         *tailcatDERP
	node         *tailcatNode
}

func NewTailcatInbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.TailcatInboundOptions) (adapter.Inbound, error) {
	privateKey, err := DecodeTailcatKey(options.PrivateKey)
	if err != nil {
		return nil, E.Cause(err, "parse private_key")
	}
	var presharedKey device.NoisePresharedKey
	if options.PreSharedKey != "" {
		presharedKey, err = DecodeTailcatKey(options.PreSharedKey)
		if err != nil {
			return nil, E.Cause(err, "parse pre_shared_key")
		}
	}
	var users map[key.NodePublic]string
	if len(options.Users) > 0 {
		users = make(map[key.NodePublic]string, len(options.Users))
		for index, user := range options.Users {
			publicKey, err := DecodeTailcatKey(user.PublicKey)
			if err != nil {
				return nil, E.Cause(err, "parse users[", index, "].public_key")
			}
			users[key.NodePublicFromRaw32(mem.B(publicKey[:]))] = user.Name
		}
	}
	derp, err := newTailcatDERP(options.TailcatDERPOptions, "server")
	if err != nil {
		return nil, err
	}
	outboundDialer, err := dialer.NewWithOptions(dialer.Options{
		Context:          ctx,
		Options:          options.DialerOptions,
		RemoteIsDomain:   true,
		ResolverOnDetour: true,
		NewDialer:        true,
	})
	if err != nil {
		return nil, err
	}
	return &TailcatInbound{
		Adapter:      inbound.NewAdapter(C.TypeTailcat, tag),
		ctx:          ctx,
		router:       router,
		logger:       logger,
		dnsRouter:    service.FromContext[adapter.DNSRouter](ctx),
		dialer:       outboundDialer,
		queryOptions: outboundDialer.(dialer.ResolveDialer).QueryOptions(),
		detour:       options.Detour,
		privateKey:   tailcatNodePrivate(privateKey),
		presharedKey: presharedKey,
		users:        users,
		derp:         derp,
	}, nil
}

func (i *TailcatInbound) PublicKey() key.NodePublic {
	return i.privateKey.Public()
}

func (i *TailcatInbound) UserPublicKeys() []key.NodePublic {
	return slices.Collect(maps.Keys(i.users))
}

func (i *TailcatInbound) References() []string {
	if i.detour == "" {
		return nil
	}
	return []string{i.detour}
}

func (i *TailcatInbound) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStatePostStart {
		return nil
	}
	err := i.derp.start(i.ctx, i.logger)
	if err != nil {
		return err
	}
	region, err := i.derp.resolve(i.ctx)
	if err != nil {
		return err
	}
	binding, err := newSystemBinding(i.ctx, i.logger)
	if err != nil {
		return err
	}
	node, err := newTailcatNode(tailcatNodeOptions{
		Context:      i.ctx,
		Logger:       i.logger,
		PrivateKey:   i.privateKey,
		PresharedKey: i.presharedKey,
		Region:       region,
		Hooks:        binding.hooks(i.dialer),
		LookupHook: func(ctx context.Context, host string) ([]netip.Addr, error) {
			return i.dnsRouter.Lookup(ctx, host, i.queryOptions)
		},
		MemoryPressure: oomkiller.MemoryPressure(i.ctx),
		Handler:        i,
		Users:          i.users,
	})
	if err != nil {
		return err
	}
	err = node.start()
	if err != nil {
		node.close()
		return err
	}
	i.node = node
	return nil
}

func (i *TailcatInbound) Close() error {
	if i.node != nil {
		i.node.close()
		i.node = nil
	}
	return nil
}

func (i *TailcatInbound) InterfaceUpdated(ctx context.Context) {
	if i.node != nil {
		i.node.interfaceUpdated()
	}
}

// Connections to the server's own address are served locally, like the
// Tailscale endpoint does; everything else is exit-node traffic, whose
// IPv4 destinations arrive NAT64-mapped.
func (i *TailcatInbound) rewriteDestination(destination netip.Addr) netip.Addr {
	if destination == i.node.address {
		return netip.IPv6Loopback()
	}
	return tailcatUnmapNAT64(destination)
}

func (i *TailcatInbound) JudgeFlow(network uint8, source netip.AddrPort, destination netip.AddrPort, firstPacket []byte) tun.FlowVerdict {
	if i.rewriteDestination(destination.Addr()) != destination.Addr() {
		return tun.FlowVerdict{Action: tun.ActionAccept}
	}
	return adapter.JudgeFlow(i.router, adapter.InboundContext{
		Inbound:     i.Tag(),
		InboundType: i.Type(),
		User:        i.node.userName(source.Addr()),
	}, network, source, destination, firstPacket)
}

func (i *TailcatInbound) NewDNSPacket(payload []byte, source M.Socksaddr, destination M.Socksaddr, writer N.PacketWriter) {
	ctx := log.ContextWithNewID(i.ctx)
	var metadata adapter.InboundContext
	metadata.Inbound = i.Tag()
	metadata.InboundType = i.Type()
	metadata.Network = N.NetworkUDP
	metadata.Source = source
	metadata.Destination = destination
	metadata.User = i.node.userName(source.Addr)
	metadata.Protocol = C.ProtocolDNS
	i.logger.InfoContext(ctx, "inbound DNS packet from ", source)
	i.router.HijackDNSPacket(ctx, payload, writer, metadata)
}

func (i *TailcatInbound) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	ctx = log.ContextWithNewID(ctx)
	var metadata adapter.InboundContext
	metadata.Inbound = i.Tag()
	metadata.InboundType = i.Type()
	metadata.Source = source
	metadata.User = i.node.userName(source.Addr)
	rewritten := i.rewriteDestination(destination.Addr)
	if rewritten != destination.Addr {
		metadata.OriginDestination = destination
		destination.Addr = rewritten
	}
	metadata.Destination = destination
	i.logger.InfoContext(ctx, "inbound connection from ", source)
	i.logger.InfoContext(ctx, "inbound connection to ", metadata.Destination)
	i.router.RouteConnectionEx(ctx, conn, metadata, onClose)
}

func (i *TailcatInbound) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	ctx = log.ContextWithNewID(ctx)
	var metadata adapter.InboundContext
	metadata.Inbound = i.Tag()
	metadata.InboundType = i.Type()
	metadata.Source = source
	metadata.User = i.node.userName(source.Addr)
	rewritten := i.rewriteDestination(destination.Addr)
	if rewritten != destination.Addr {
		metadata.OriginDestination = destination
		destination.Addr = rewritten
		conn = bufio.NewNATPacketConn(bufio.NewNetPacketConn(conn), metadata.OriginDestination, destination)
	}
	metadata.Destination = destination
	i.logger.InfoContext(ctx, "inbound packet connection from ", source)
	i.logger.InfoContext(ctx, "inbound packet connection to ", metadata.Destination)
	i.router.RoutePacketConnectionEx(ctx, conn, metadata, onClose)
}
