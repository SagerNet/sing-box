package masque

import (
	"context"
	"io"
	"math"
	"net"
	"net/netip"
	"slices"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/endpoint"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/iponly"
	"github.com/sagernet/sing-box/common/listener"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/service/oomkiller"
	"github.com/sagernet/sing-box/transport/device"
	"github.com/sagernet/sing-box/transport/http"
	"github.com/sagernet/sing-box/transport/masque"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

var (
	_ adapter.OutboundWithPreferredRoutes = (*ServerEndpoint)(nil)
	_ adapter.FlowOutbound                = (*ServerEndpoint)(nil)
	_ adapter.ConnectionHandler           = (*serverConnectionHandler)(nil)
	_ dialer.PacketDialerWithDestination  = (*ServerEndpoint)(nil)
	_ masque.ServerHandler                = (*ServerEndpoint)(nil)
)

type ServerEndpoint struct {
	endpointBase
	ctx            context.Context
	dnsRouter      adapter.DNSRouter
	listener       *listener.Listener
	httpServer     *http.Server
	tlsConfig      tls.ServerConfig
	http3          bool
	quicOptions    option.QUICOptions
	http3Server    io.Closer
	server         *masque.Server
	deviceOptions  *device.Options
	device         device.Device
	localAddresses []netip.Prefix
	started        atomic.Bool
}

func NewServerEndpoint(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.MASQUEServerEndpointOptions) (adapter.Endpoint, error) {
	if options.MTU == 0 {
		options.MTU = masque.DefaultMTU
	}
	versions := options.Versions()
	if len(options.Version) == 0 && http.ConfigureHTTP3ListenerFunc == nil {
		logger.Warn("QUIC is not included in this build, HTTP/3 is disabled")
		versions = []int{1, 2}
	}
	serveHTTP1 := slices.Contains(versions, 1)
	serveHTTP2 := slices.Contains(versions, 2)
	serveHTTP3 := slices.Contains(versions, 3)
	if serveHTTP3 && (options.TLS == nil || !options.TLS.Enabled) {
		return nil, E.New("TLS is required for HTTP/3")
	}
	if options.HTTP3Options.InitialPacketSize == 0 {
		options.HTTP3Options.InitialPacketSize = min(int(options.MTU)+masque.QUICPacketOverhead, math.MaxUint16)
	}
	serverEndpoint := &ServerEndpoint{
		endpointBase: endpointBase{
			Adapter: endpoint.NewAdapter(C.TypeMASQUEServer, tag, []string{N.NetworkTCP, N.NetworkUDP, N.NetworkICMP}, nil),
			router:  router,
			logger:  logger,
		},
		ctx:            ctx,
		dnsRouter:      service.FromContext[adapter.DNSRouter](ctx),
		http3:          serveHTTP3,
		quicOptions:    options.HTTP3Options,
		localAddresses: options.Address,
	}
	server, err := masque.NewServer(masque.ServerOptions{
		Context:         ctx,
		Logger:          logger,
		Path:            options.Path,
		Address:         options.Address,
		AdvertiseRoutes: options.AdvertiseRoutes,
		Resolve:         serverEndpoint.resolve,
		Handler:         serverEndpoint,
	})
	if err != nil {
		return nil, err
	}
	serverEndpoint.server = server
	serverEndpoint.httpServer = http.NewServer(http.ServerOptions{
		Authenticator: auth.NewAuthenticator(options.Users),
		Logger:        logger,
		HTTP1:         serveHTTP1,
		HTTP2:         serveHTTP2,
		HTTP2Options:  options.HTTP2Options,
		Tunnels:       map[string]http.TunnelHandler{"connect-ip": server},
	})
	if options.TLS != nil {
		tlsConfig, tlsErr := tls.NewServerWithOptions(tls.ServerOptions{
			Context:        ctx,
			Logger:         logger,
			Options:        common.PtrValueOrDefault(options.TLS),
			KTLSCompatible: true,
		})
		if tlsErr != nil {
			return nil, tlsErr
		}
		if tlsConfig != nil {
			serverEndpoint.httpServer.ConfigureTLS(tlsConfig)
		}
		serverEndpoint.tlsConfig = tlsConfig
	}
	var network []string
	if serveHTTP1 || serveHTTP2 {
		network = []string{N.NetworkTCP}
	}
	serverEndpoint.listener = listener.New(listener.Options{
		Context:           ctx,
		Logger:            logger,
		Network:           network,
		Listen:            options.ListenOptions,
		ConnectionHandler: (*serverConnectionHandler)(serverEndpoint),
	})
	serverEndpoint.deviceOptions = newDeviceOptions(ctx, logger, serverEndpoint, options.MASQUEEndpointOptions, time.Duration(options.UDPTimeout), options.Address)
	return serverEndpoint, nil
}

func (s *ServerEndpoint) resolve(ctx context.Context, domain string) ([]netip.Addr, error) {
	return s.dnsRouter.Lookup(ctx, domain, adapter.DNSQueryOptions{})
}

func (s *ServerEndpoint) Start(stage adapter.StartStage) error {
	switch stage {
	case adapter.StartStateInitialize:
		s.deviceOptions.MemoryPressure = oomkiller.MemoryPressure(s.ctx)
		tunnelDevice, err := device.New(*s.deviceOptions)
		if err != nil {
			return err
		}
		tunnelDevice.SetPacketWriter(s.writePacketBuffers)
		s.device = tunnelDevice
		s.deviceOptions = nil
	case adapter.StartStateStart:
		if s.tlsConfig != nil {
			err := s.tlsConfig.Start()
			if err != nil {
				return E.Cause(err, "create TLS config")
			}
		}
		err := s.device.Start()
		if err != nil {
			return E.Cause(err, "start device")
		}
		err = s.listener.Start()
		if err != nil {
			return err
		}
		if s.http3 {
			s.http3Server, err = s.httpServer.ListenHTTP3(s.ctx, s.logger, s.listener, nil, s.tlsConfig, s.quicOptions)
			if err != nil {
				return err
			}
		}
		s.started.Store(true)
	}
	return nil
}

func (s *ServerEndpoint) Close() error {
	s.started.Store(false)
	return common.Close(
		s.listener,
		s.http3Server,
		s.server,
		s.device,
		s.tlsConfig,
	)
}

type serverConnectionHandler ServerEndpoint

func (h *serverConnectionHandler) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	if h.tlsConfig != nil {
		tlsConn, err := tls.ServerHandshake(ctx, conn, h.tlsConfig)
		if err != nil {
			N.CloseOnHandshakeFailure(conn, onClose, err)
			h.logger.ErrorContext(ctx, E.Cause(err, "process connection from ", metadata.Source, ": TLS handshake"))
			return
		}
		conn = tlsConn
	}
	h.httpServer.ServeConnection(ctx, conn, http.NewReader(conn), nil, metadata.Source, onClose)
}

func (s *ServerEndpoint) WriteInboundBuffers(packetBuffers []*buf.Buffer) error {
	err := s.device.WriteInboundBuffers(packetBuffers)
	buf.ReleaseMulti(packetBuffers)
	return err
}

func (s *ServerEndpoint) PreMatchFlow(network string, destination netip.Addr) adapter.PreMatchAction {
	return adapter.PreMatchFlow
}

func (s *ServerEndpoint) PortAddresses() (netip.Addr, netip.Addr) {
	return s.device.PortAddresses()
}

func (s *ServerEndpoint) PortMTU() uint32 {
	return s.device.PortMTU()
}

func (s *ServerEndpoint) AttachReturn(returnPath tun.Return) error {
	return s.device.AttachReturn(returnPath)
}

func (s *ServerEndpoint) DetachReturn(returnPath tun.Return) error {
	return s.device.DetachReturn(returnPath)
}

func (s *ServerEndpoint) JudgeFlow(network uint8, source netip.AddrPort, destination netip.AddrPort, firstPacket []byte) tun.FlowVerdict {
	return s.judgeFlow(s, s.localAddresses, network, source, destination, firstPacket)
}

func (s *ServerEndpoint) NewDNSPacket(payload []byte, source M.Socksaddr, destination M.Socksaddr, writer N.PacketWriter) {
	s.newDNSPacket(log.ContextWithNewID(s.ctx), s, payload, source, destination, writer)
}

func (s *ServerEndpoint) WritePackets(packets [][]byte) error {
	if !s.started.Load() {
		return E.New("endpoint is not ready yet")
	}
	return s.server.WritePacketBuffers(common.Map(packets, buf.As), true)
}

func (s *ServerEndpoint) writePacketBuffers(packetBuffers []*buf.Buffer) error {
	return s.server.WritePacketBuffers(packetBuffers, false)
}

func (s *ServerEndpoint) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	s.newConnection(ctx, s, s.localAddresses, conn, source, destination, onClose)
}

func (s *ServerEndpoint) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	s.newPacketConnection(ctx, s, s.localAddresses, conn, source, destination, onClose)
}

func (s *ServerEndpoint) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	switch network {
	case N.NetworkTCP:
		s.logger.InfoContext(ctx, "outbound connection to ", destination)
	case N.NetworkUDP:
		s.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	}
	if !s.started.Load() {
		return nil, E.New("endpoint is not ready yet")
	}
	if destination.IsDomain() {
		destinationAddresses, err := s.dnsRouter.Lookup(ctx, destination.Fqdn, adapter.DNSQueryOptions{})
		if err != nil {
			return nil, err
		}
		return N.DialSerial(ctx, s.device, network, destination, destinationAddresses)
	}
	if !destination.Addr.IsValid() {
		return nil, E.New("invalid destination: ", destination)
	}
	return s.device.DialContext(ctx, network, destination)
}

func (s *ServerEndpoint) ListenPacketWithDestination(ctx context.Context, destination M.Socksaddr) (net.PacketConn, netip.Addr, error) {
	s.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	if !s.started.Load() {
		return nil, netip.Addr{}, E.New("endpoint is not ready yet")
	}
	if destination.IsDomain() {
		destinationAddresses, err := s.dnsRouter.Lookup(ctx, destination.Fqdn, adapter.DNSQueryOptions{})
		if err != nil {
			return nil, netip.Addr{}, err
		}
		packetConn, destinationAddress, err := N.ListenSerial(ctx, s.device, destination, destinationAddresses)
		if err != nil {
			return nil, netip.Addr{}, err
		}
		return iponly.NewPacketConn(s.logger, packetConn), destinationAddress, nil
	}
	packetConn, err := s.device.ListenPacket(ctx, destination)
	if err != nil {
		return nil, netip.Addr{}, err
	}
	if destination.IsIP() {
		return iponly.NewPacketConn(s.logger, packetConn), destination.Addr, nil
	}
	return iponly.NewPacketConn(s.logger, packetConn), netip.Addr{}, nil
}

func (s *ServerEndpoint) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	packetConn, destinationAddress, err := s.ListenPacketWithDestination(ctx, destination)
	if err != nil {
		return nil, err
	}
	if destinationAddress.IsValid() && destination != M.SocksaddrFrom(destinationAddress, destination.Port) {
		return bufio.NewNATPacketConn(bufio.NewPacketConn(packetConn), M.SocksaddrFrom(destinationAddress, destination.Port), destination), nil
	}
	return packetConn, nil
}

func (s *ServerEndpoint) PreferredDomain(metadata *adapter.InboundContext, domain string) bool {
	return false
}

func (s *ServerEndpoint) PreferredAddress(metadata *adapter.InboundContext, address netip.Addr) bool {
	return s.started.Load() && s.server.Contains(address)
}
