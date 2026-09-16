//go:build with_tailscale

package tailscale

import (
	"context"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/iponly"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/service/oomkiller"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/tailscale/types/key"
	"github.com/sagernet/wireguard-go/device"

	"go4.org/mem"
)

var (
	_ adapter.Outbound                   = (*TailcatOutbound)(nil)
	_ adapter.IdleConnectionKeeper       = (*TailcatOutbound)(nil)
	_ adapter.InterfaceUpdateListener    = (*TailcatOutbound)(nil)
	_ dialer.PacketDialerWithDestination = (*TailcatOutbound)(nil)
)

func RegisterTailcatOutbound(registry *outbound.Registry) {
	outbound.Register[option.TailcatOutboundOptions](registry, C.TypeTailcat, NewTailcatOutbound)
}

type TailcatOutbound struct {
	outbound.Adapter
	ctx             context.Context
	logger          logger.ContextLogger
	dnsRouter       adapter.DNSRouter
	dialer          N.Dialer
	queryOptions    adapter.DNSQueryOptions
	privateKey      key.NodePrivate
	serverPublicKey key.NodePublic
	serverDiscoKey  key.DiscoPublic
	presharedKey    device.NoisePresharedKey
	derp            *tailcatDERP
	access          sync.Mutex
	node            *tailcatNode
	active          int
	idle            bool
	closed          bool
}

func NewTailcatOutbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.TailcatOutboundOptions) (adapter.Outbound, error) {
	var privateKey [32]byte
	if options.PrivateKey != "" {
		var err error
		privateKey, err = DecodeTailcatKey(options.PrivateKey)
		if err != nil {
			return nil, E.Cause(err, "parse private_key")
		}
	} else {
		privateKey = NewTailcatPrivateKey()
	}
	serverPublicKey, err := DecodeTailcatKey(options.ServerPublicKey)
	if err != nil {
		return nil, E.Cause(err, "parse server_public_key")
	}
	serverDiscoKey, err := DecodeTailcatKey(options.ServerDiscoKey)
	if err != nil {
		return nil, E.Cause(err, "parse server_disco_key")
	}
	var presharedKey device.NoisePresharedKey
	if options.PreSharedKey != "" {
		presharedKey, err = DecodeTailcatKey(options.PreSharedKey)
		if err != nil {
			return nil, E.Cause(err, "parse pre_shared_key")
		}
	}
	derp, err := newTailcatDERP(options.TailcatDERPOptions, "client")
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
	return &TailcatOutbound{
		Adapter:         outbound.NewAdapterWithDialerOptions(C.TypeTailcat, tag, []string{N.NetworkTCP, N.NetworkUDP}, options.DialerOptions),
		ctx:             ctx,
		logger:          logger,
		dnsRouter:       service.FromContext[adapter.DNSRouter](ctx),
		dialer:          outboundDialer,
		queryOptions:    outboundDialer.(dialer.ResolveDialer).QueryOptions(),
		privateKey:      tailcatNodePrivate(privateKey),
		serverPublicKey: key.NodePublicFromRaw32(mem.B(serverPublicKey[:])),
		serverDiscoKey:  key.DiscoPublicFromRaw32(mem.B(serverDiscoKey[:])),
		presharedKey:    presharedKey,
		derp:            derp,
	}, nil
}

func (o *TailcatOutbound) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStatePostStart {
		return nil
	}
	return o.derp.start(o.ctx, o.logger)
}

func (o *TailcatOutbound) Close() error {
	o.access.Lock()
	defer o.access.Unlock()
	o.closed = true
	o.closeNodeLocked()
	return nil
}

func (o *TailcatOutbound) closeNodeLocked() {
	if o.node == nil {
		return
	}
	o.logger.Info("disconnected from server")
	o.node.close()
	o.node = nil
}

func (o *TailcatOutbound) acquire(ctx context.Context) (*tailcatNode, func(), error) {
	o.access.Lock()
	defer o.access.Unlock()
	if o.closed {
		return nil, nil, net.ErrClosed
	}
	node, err := o.ensureLocked(ctx)
	if err != nil {
		return nil, nil, err
	}
	o.active++
	return node, sync.OnceFunc(func() {
		o.access.Lock()
		defer o.access.Unlock()
		o.active--
		if o.active == 0 && o.idle {
			o.closeNodeLocked()
		}
	}), nil
}

func (o *TailcatOutbound) ensureLocked(ctx context.Context) (*tailcatNode, error) {
	if o.node != nil {
		o.node.requestMeow()
		return o.node, nil
	}
	region, err := o.derp.resolve(ctx)
	if err != nil {
		return nil, err
	}
	binding, err := newSystemBinding(o.ctx, o.logger)
	if err != nil {
		return nil, err
	}
	node, err := newTailcatNode(tailcatNodeOptions{
		Context:      o.ctx,
		Logger:       o.logger,
		PrivateKey:   o.privateKey,
		PresharedKey: o.presharedKey,
		Region:       region,
		Hooks:        binding.hooks(o.dialer),
		LookupHook: func(ctx context.Context, host string) ([]netip.Addr, error) {
			return o.dnsRouter.Lookup(ctx, host, o.queryOptions)
		},
		MemoryPressure:  oomkiller.MemoryPressure(o.ctx),
		ServerPublicKey: o.serverPublicKey,
		ServerDiscoKey:  o.serverDiscoKey,
	})
	if err != nil {
		return nil, err
	}
	err = node.start()
	if err != nil {
		node.close()
		return nil, err
	}
	err = node.meow(ctx)
	if err != nil {
		node.close()
		return nil, E.Cause(err, "connect to server")
	}
	o.logger.Info("connected to server")
	o.node = node
	return node, nil
}

func (o *TailcatOutbound) SetKeepIdleConnections(keep bool) {
	o.access.Lock()
	defer o.access.Unlock()
	o.idle = !keep
	if o.idle && o.active == 0 {
		o.closeNodeLocked()
	}
}

func (o *TailcatOutbound) CloseIdleConnections() {
	o.access.Lock()
	defer o.access.Unlock()
	if o.active == 0 {
		o.closeNodeLocked()
	}
}

func (o *TailcatOutbound) InterfaceUpdated(ctx context.Context) {
	o.access.Lock()
	defer o.access.Unlock()
	if o.node != nil {
		o.node.interfaceUpdated()
	}
}

func (o *TailcatOutbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		o.logger.InfoContext(ctx, "outbound connection to ", destination)
	case N.NetworkUDP:
		o.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	default:
		return nil, E.Extend(N.ErrUnknownNetwork, network)
	}
	if destination.IsDomain() {
		destinationAddresses, err := o.dnsRouter.Lookup(ctx, destination.Fqdn, adapter.DNSQueryOptions{})
		if err != nil {
			return nil, err
		}
		return N.DialSerial(ctx, (*tailcatDialer)(o), network, destination, destinationAddresses)
	}
	return (*tailcatDialer)(o).DialContext(ctx, network, destination)
}

func (o *TailcatOutbound) ListenPacketWithDestination(ctx context.Context, destination M.Socksaddr) (net.PacketConn, netip.Addr, error) {
	o.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	if destination.IsDomain() {
		destinationAddresses, err := o.dnsRouter.Lookup(ctx, destination.Fqdn, adapter.DNSQueryOptions{})
		if err != nil {
			return nil, netip.Addr{}, err
		}
		packetConn, destinationAddress, err := N.ListenSerial(ctx, (*tailcatDialer)(o), destination, destinationAddresses)
		if err != nil {
			return nil, netip.Addr{}, err
		}
		return iponly.NewPacketConn(o.logger, packetConn), destinationAddress, nil
	}
	packetConn, err := (*tailcatDialer)(o).ListenPacket(ctx, destination)
	if err != nil {
		return nil, netip.Addr{}, err
	}
	if destination.IsIP() {
		return iponly.NewPacketConn(o.logger, packetConn), destination.Addr, nil
	}
	return iponly.NewPacketConn(o.logger, packetConn), netip.Addr{}, nil
}

func (o *TailcatOutbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	packetConn, destinationAddress, err := o.ListenPacketWithDestination(ctx, destination)
	if err != nil {
		return nil, err
	}
	if destinationAddress.IsValid() && destination != M.SocksaddrFrom(destinationAddress, destination.Port) {
		return bufio.NewNATPacketConn(bufio.NewPacketConn(packetConn), M.SocksaddrFrom(destinationAddress, destination.Port), destination), nil
	}
	return packetConn, nil
}

// tailcatDialer is the IP-only dialer behind the outbound: it brings
// the tunnel up, maps IPv4 destinations into the NAT64 prefix that the
// server unmaps, and tracks live connections for the idle teardown.
type tailcatDialer TailcatOutbound

func (d *tailcatDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if !destination.Addr.IsValid() {
		return nil, E.New("invalid destination: ", destination)
	}
	node, release, err := (*TailcatOutbound)(d).acquire(ctx)
	if err != nil {
		return nil, err
	}
	mapped := netip.AddrPortFrom(tailcatMapNAT64(destination.Addr), destination.Port)
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		conn, err := node.stack().DialTCP(ctx, node.address, mapped)
		if err != nil {
			release()
			return nil, err
		}
		return &tailcatConn{GoConn: conn, release: release}, nil
	case N.NetworkUDP:
		conn, err := node.stack().DialUDP(netip.AddrPortFrom(node.address, 0), mapped)
		if err != nil {
			release()
			return nil, err
		}
		return &tailcatPacketConn{conn: conn, remote: destination.UDPAddr(), release: release}, nil
	default:
		release()
		return nil, E.Extend(N.ErrUnknownNetwork, network)
	}
}

func (d *tailcatDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	node, release, err := (*TailcatOutbound)(d).acquire(ctx)
	if err != nil {
		return nil, err
	}
	conn, err := node.stack().ListenUDP(netip.AddrPortFrom(node.address, 0))
	if err != nil {
		release()
		return nil, err
	}
	return &tailcatPacketConn{conn: conn, release: release}, nil
}

type tailcatConn struct {
	*tun.GoConn
	release func()
}

func (c *tailcatConn) Close() error {
	err := c.GoConn.Close()
	c.release()
	return err
}

func (c *tailcatConn) Upstream() any {
	return c.GoConn
}

// GoUDPConn also offers buffer-based ReadPacket/WritePacket, which
// bufio prefers over ReadFrom/WriteTo, so every address-carrying
// method is wrapped rather than the conn being embedded.
type tailcatPacketConn struct {
	conn    *tun.GoUDPConn
	remote  net.Addr
	release func()
}

func (c *tailcatPacketConn) Read(buffer []byte) (int, error) {
	return c.conn.Read(buffer)
}

func (c *tailcatPacketConn) Write(buffer []byte) (int, error) {
	return c.conn.Write(buffer)
}

func (c *tailcatPacketConn) ReadFrom(buffer []byte) (int, net.Addr, error) {
	n, source, err := c.conn.ReadFromUDPAddrPort(buffer)
	if err != nil {
		return n, nil, err
	}
	return n, net.UDPAddrFromAddrPort(netip.AddrPortFrom(tailcatUnmapNAT64(source.Addr()), source.Port())), nil
}

func (c *tailcatPacketConn) WriteTo(buffer []byte, addr net.Addr) (int, error) {
	destination := M.SocksaddrFromNet(addr)
	if !destination.Addr.IsValid() {
		return 0, E.New("invalid destination: ", addr)
	}
	return c.conn.WriteToUDPAddrPort(buffer, netip.AddrPortFrom(tailcatMapNAT64(destination.Addr), destination.Port))
}

func (c *tailcatPacketConn) ReadPacket(buffer *buf.Buffer) (M.Socksaddr, error) {
	source, err := c.conn.ReadPacket(buffer)
	if err != nil {
		return M.Socksaddr{}, err
	}
	source.Addr = tailcatUnmapNAT64(source.Addr)
	return source, nil
}

func (c *tailcatPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	if !destination.Addr.IsValid() {
		buffer.Release()
		return E.New("invalid destination: ", destination)
	}
	destination.Addr = tailcatMapNAT64(destination.Addr)
	return c.conn.WritePacket(buffer, destination)
}

func (c *tailcatPacketConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *tailcatPacketConn) RemoteAddr() net.Addr {
	if c.remote != nil {
		return c.remote
	}
	return c.conn.RemoteAddr()
}

func (c *tailcatPacketConn) SetDeadline(deadline time.Time) error {
	return c.conn.SetDeadline(deadline)
}

func (c *tailcatPacketConn) SetReadDeadline(deadline time.Time) error {
	return c.conn.SetReadDeadline(deadline)
}

func (c *tailcatPacketConn) SetWriteDeadline(deadline time.Time) error {
	return c.conn.SetWriteDeadline(deadline)
}

func (c *tailcatPacketConn) Close() error {
	err := c.conn.Close()
	c.release()
	return err
}
