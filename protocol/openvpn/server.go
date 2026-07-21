package openvpn

import (
	"context"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/endpoint"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/listener"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	ovpntransport "github.com/sagernet/sing-box/transport/openvpn"
	ovpn "github.com/sagernet/sing-openvpn"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

var (
	_ adapter.FlowOutbound               = (*ServerEndpoint)(nil)
	_ dialer.PacketDialerWithDestination = (*ServerEndpoint)(nil)
)

type ServerEndpoint struct {
	endpointBase
	ctx            context.Context
	loopContext    context.Context
	cancelLoop     context.CancelFunc
	options        option.OpenVPNServerEndpointOptions
	serverOptions  ovpn.ServerOptions
	dnsRouter      adapter.DNSRouter
	listener       *listener.Listener
	server         *ovpn.Server
	device         ovpntransport.Device
	localAddresses []netip.Prefix
	started        atomic.Bool
	readLoopDone   chan struct{}
}

type udpEgressPacketConn struct {
	*tun.UDPEgressConn
}

func (c *udpEgressPacketConn) ReadFrom(buffer []byte) (int, net.Addr, error) {
	dataLength, source, err := c.ReadFromUDPAddrPort(buffer)
	if err != nil {
		return 0, nil, err
	}
	return dataLength, net.UDPAddrFromAddrPort(source), nil
}

func (c *udpEgressPacketConn) WriteTo(buffer []byte, destination net.Addr) (int, error) {
	destinationAddress := M.SocksaddrFromNet(destination)
	if !destinationAddress.IsIP() {
		return 0, E.New("invalid UDP destination: ", destination)
	}
	return c.WriteToUDPAddrPort(buffer, destinationAddress.AddrPort())
}

func NewServerEndpoint(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.OpenVPNServerEndpointOptions) (adapter.Endpoint, error) {
	if options.MTU == 0 {
		options.MTU = ovpntransport.DefaultMTU
	}
	loopContext, cancelLoop := context.WithCancel(ctx)
	serverEndpoint := &ServerEndpoint{
		endpointBase: endpointBase{
			Adapter: endpoint.NewAdapter(C.TypeOpenVPNServer, tag, []string{N.NetworkTCP, N.NetworkUDP, N.NetworkICMP}, nil),
			router:  router,
			logger:  logger,
		},
		ctx:            ctx,
		loopContext:    loopContext,
		cancelLoop:     cancelLoop,
		options:        options,
		dnsRouter:      service.FromContext[adapter.DNSRouter](ctx),
		localAddresses: options.Address,
	}
	serverOptions, err := buildServerOptions(options)
	if err != nil {
		cancelLoop()
		return nil, err
	}
	serverOptions.Context = loopContext
	if serverOptions.Mode == ovpn.ModeTLS {
		serverOptions.Authentication.Authenticator = authenticatorFromUsers(options.Users)
		serverOptions.Authentication.DuplicateCN = options.DuplicateCN
	}
	serverOptions.Logger = logger
	serverEndpoint.serverOptions = serverOptions
	udpTimeout := C.UDPTimeout
	if options.UDPTimeout != 0 {
		udpTimeout = time.Duration(options.UDPTimeout)
	}
	device, err := ovpntransport.NewDevice(ovpntransport.DeviceOptions{
		Context:         ctx,
		Logger:          logger,
		System:          options.System,
		Handler:         serverEndpoint,
		UDPTimeout:      udpTimeout,
		ICMPTimeout:     C.ICMPTimeout,
		UDPMapping:      tun.NATMapping(options.UDPMapping),
		UDPFiltering:    tun.NATFiltering(options.UDPFiltering),
		UDPNATMax:       options.UDPNATMax,
		InterfaceFinder: service.FromContext[adapter.NetworkManager](ctx).InterfaceFinder(),
		Name:            options.Name,
		MTU:             options.MTU,
		Configuration: ovpntransport.Configuration{
			MTU:      options.MTU,
			Address:  options.Address,
			Topology: options.Topology,
		},
	})
	if err != nil {
		cancelLoop()
		return nil, err
	}
	serverEndpoint.device = device
	device.SetPacketWriter(serverEndpoint.writePacketBuffersByDestination)
	return serverEndpoint, nil
}

func validateServerAddresses(addresses []netip.Prefix) error {
	var hasIPv4 bool
	var hasIPv6 bool
	for addressIndex, prefix := range addresses {
		if !prefix.IsValid() {
			return E.New("server address[", addressIndex, "] is invalid")
		}
		if prefix.Addr().Is4() {
			if hasIPv4 {
				return E.New("multiple IPv4 server address pools are not supported")
			}
			hasIPv4 = true
		} else {
			if hasIPv6 {
				return E.New("multiple IPv6 server address pools are not supported")
			}
			hasIPv6 = true
		}
	}
	return nil
}

func validateServerTopology(topology string) error {
	switch topology {
	case "", "subnet", "p2p", "net30":
		return nil
	default:
		return E.New("invalid topology ", topology, ", allowed values: subnet, p2p, net30")
	}
}

func (s *ServerEndpoint) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	protocol := s.serverOptions.Transport.Protocol
	s.listener = listener.New(listener.Options{
		Context: s.ctx,
		Logger:  s.logger,
		Network: []string{protocol},
		Listen:  s.options.ListenOptions,
	})
	var (
		streamListener net.Listener
		packetConn     net.PacketConn
		err            error
	)
	if protocol == N.NetworkTCP {
		streamListener, err = s.listener.ListenTCP()
	} else {
		var listenConfig net.ListenConfig
		var egressEnabled bool
		listenAddress := s.options.Listen.Build(netip.AddrFrom4([4]byte{127, 0, 0, 1}))
		if listenAddress.IsUnspecified() && s.options.BindInterface == "" && s.options.RoutingMark == 0 && s.options.NetNs == "" {
			udpDialer, dialerErr := dialer.NewDefault(s.ctx, option.DialerOptions{
				ReuseAddr:          s.options.ReuseAddr,
				UDPFragment:        s.options.UDPFragment,
				UDPFragmentDefault: s.options.UDPFragmentDefault,
			})
			if dialerErr != nil {
				return dialerErr
			}
			listenConfig.Control, egressEnabled = udpDialer.UDPListenerControl()
		}
		packetConn, err = s.listener.ListenUDPWithConfig(listenConfig)
		if err == nil {
			tuneOpenVPNUDPSocket(packetConn)
			if egressEnabled {
				udpConn := packetConn.(*net.UDPConn)
				networkManager := service.FromContext[adapter.NetworkManager](s.ctx)
				egressPool := tun.NewUDPEgressPool(tun.UDPEgressPoolOptions{
					Logger:           s.logger,
					Network:          M.NetworkFromNetAddr(N.NetworkUDP, listenAddress),
					Control:          listenConfig.Control,
					InterfaceFinder:  networkManager.InterfaceFinder(),
					InterfaceMonitor: networkManager.InterfaceMonitor(),
					ExcludeInterface: s.options.Name,
					IsExempt: func() bool {
						return networkManager.AutoRedirectOutputMark() != 0
					},
				})
				listenPort := udpConn.LocalAddr().(*net.UDPAddr).AddrPort().Port()
				if egressPool.SetEgressPort(listenPort) {
					packetConn = &udpEgressPacketConn{tun.NewUDPEgressConn(udpConn, egressPool)}
				} else {
					egressPool.Close()
				}
			}
		}
	}
	if err != nil {
		return err
	}
	serverOptions := s.serverOptions
	if streamListener != nil {
		serverOptions.Transport.ListenAddress = streamListener.Addr().String()
	} else if packetConn != nil {
		serverOptions.Transport.ListenAddress = packetConn.LocalAddr().String()
	}
	serverOptions.Transport.Listener = streamListener
	serverOptions.Transport.PacketConn = packetConn
	server, err := ovpn.NewServer(serverOptions)
	if err != nil {
		if packetConn != nil {
			_ = packetConn.Close()
		}
		s.listener.Close()
		return err
	}
	s.server = server
	err = s.device.Start()
	if err != nil {
		s.listener.Close()
		server.Close()
		return err
	}
	err = server.Start()
	if err != nil {
		s.device.Close()
		s.listener.Close()
		server.Close()
		return err
	}
	s.started.Store(true)
	s.readLoopDone = make(chan struct{})
	go s.readLoop()
	return nil
}

func buildServerOptions(options option.OpenVPNServerEndpointOptions) (ovpn.ServerOptions, error) {
	mode := options.Mode
	if mode == "" {
		mode = ovpn.ModeTLS
	}
	switch mode {
	case ovpn.ModeTLS, ovpn.ModeStaticKey:
	default:
		return ovpn.ServerOptions{}, E.New("unsupported mode: ", mode, " (expected \"tls\" or \"static_key\")")
	}
	if len(options.Address) == 0 {
		return ovpn.ServerOptions{}, E.New("missing server address")
	}
	err := validateServerAddresses(options.Address)
	if err != nil {
		return ovpn.ServerOptions{}, err
	}
	err = validateServerTopology(options.Topology)
	if err != nil {
		return ovpn.ServerOptions{}, err
	}
	protocol := options.Network
	if protocol == "" {
		protocol = N.NetworkUDP
	}
	switch protocol {
	case N.NetworkTCP, N.NetworkUDP:
	default:
		return ovpn.ServerOptions{}, E.New("unsupported network: ", protocol)
	}
	if mode == ovpn.ModeStaticKey {
		return buildStaticKeyServerOptions(options, protocol)
	}
	if options.TLS == nil {
		return ovpn.ServerOptions{}, E.New("missing `tls` options")
	}
	if len(options.StaticKey) > 0 || options.StaticKeyPath != "" || options.KeyDirection != "" || options.Cipher != "" || options.Remote != "" || options.RemotePort != 0 || netip.Addr(options.PeerAddress).IsValid() || netip.Addr(options.PeerAddressIPv6).IsValid() {
		return ovpn.ServerOptions{}, E.New("static-key server options require `mode: static_key`")
	}
	tlsOptions, keyDirection, err := buildServerTLSOptions(*options.TLS)
	if err != nil {
		return ovpn.ServerOptions{}, err
	}
	serverOptions := ovpn.ServerOptions{
		Mode:         ovpn.ModeTLS,
		KeyDirection: keyDirection,
		Transport: ovpn.ServerTransportOptions{
			Protocol: protocol,
		},
		Resources: ovpn.ServerResourceOptions{
			MaxClients: options.MaxClients,
		},
		DataChannel: ovpn.ServerDataChannelOptions{
			MTU:              options.MTU,
			MSSFix:           options.MSSFix,
			MSSFixDisabled:   options.MSSFixDisabled,
			MSSFixMode:       options.MSSFixMode,
			Ciphers:          []string(options.DataCiphers),
			FallbackCipher:   options.DataCiphersFallback,
			Auth:             options.Auth,
			ReplayWindow:     options.ReplayWindow,
			ReplayWindowTime: time.Duration(options.ReplayWindowTime),
			PacketHeadroom:   ovpntransport.PacketHeadroom,
		},
		TLS: tlsOptions,
		Timing: ovpn.ServerTimingOptions{
			RenegotiationInterval: time.Duration(options.RenegotiateInterval),
			RenegotiationDisabled: options.RenegotiateDisabled,
			RenegotiationBytes:    options.RenegotiateBytes,
			RenegotiationPackets:  options.RenegotiatePackets,
			HandWindow:            time.Duration(options.HandshakeWindow),
			PingInterval:          time.Duration(options.PingInterval),
			PingRestart:           time.Duration(options.PingRestart),
		},
	}
	err = applyServerPushOptions(&serverOptions, options)
	if err != nil {
		return ovpn.ServerOptions{}, err
	}
	return serverOptions, nil
}

func buildStaticKeyServerOptions(options option.OpenVPNServerEndpointOptions, protocol string) (ovpn.ServerOptions, error) {
	if options.TLS != nil {
		return ovpn.ServerOptions{}, E.New("`tls` options are not supported in `static_key` mode")
	}
	if len(options.Users) > 0 || options.DuplicateCN {
		return ovpn.ServerOptions{}, E.New("user authentication is not supported in `static_key` mode")
	}
	if options.Push != nil {
		return ovpn.ServerOptions{}, E.New("push options are not supported in `static_key` mode")
	}
	if options.RenegotiateInterval != 0 || options.RenegotiateDisabled || options.RenegotiateBytes != 0 || options.RenegotiatePackets != 0 || options.HandshakeWindow != 0 {
		return ovpn.ServerOptions{}, E.New("TLS timing and renegotiation options are not supported in `static_key` mode")
	}
	if len(options.DataCiphers) > 0 || options.DataCiphersFallback != "" {
		return ovpn.ServerOptions{}, E.New("`data_ciphers` and `data_ciphers_fallback` are not supported in `static_key` mode; use `cipher`")
	}
	staticKey, err := requiredMaterialSource("static_key", options.StaticKey, options.StaticKeyPath)
	if err != nil {
		return ovpn.ServerOptions{}, err
	}
	keyDirection, err := keyDirectionValue(options.KeyDirection)
	if err != nil {
		return ovpn.ServerOptions{}, err
	}
	vpnGateway := netip.Addr(options.PeerAddress)
	if vpnGateway.IsValid() && !vpnGateway.Is4() {
		return ovpn.ServerOptions{}, E.New("`peer_address` must be an IPv4 address")
	}
	vpnGatewayIPv6 := netip.Addr(options.PeerAddressIPv6)
	if vpnGatewayIPv6.IsValid() && !vpnGatewayIPv6.Is6() {
		return ovpn.ServerOptions{}, E.New("`peer_address_ipv6` must be an IPv6 address")
	}
	var hasIPv4 bool
	var hasIPv6 bool
	for _, address := range options.Address {
		hasIPv4 = hasIPv4 || address.Addr().Is4()
		hasIPv6 = hasIPv6 || address.Addr().Is6()
	}
	if hasIPv4 && !vpnGateway.IsValid() {
		return ovpn.ServerOptions{}, E.New("missing `peer_address` for the IPv4 static-key tunnel")
	}
	if hasIPv6 && !vpnGatewayIPv6.IsValid() {
		return ovpn.ServerOptions{}, E.New("missing `peer_address_ipv6` for the IPv6 static-key tunnel")
	}
	if vpnGateway.IsValid() && !hasIPv4 {
		return ovpn.ServerOptions{}, E.New("`peer_address` requires an IPv4 tunnel `address` in `static_key` mode")
	}
	if vpnGatewayIPv6.IsValid() && !hasIPv6 {
		return ovpn.ServerOptions{}, E.New("`peer_address_ipv6` requires an IPv6 tunnel `address` in `static_key` mode")
	}
	remoteAddress := ""
	if protocol == N.NetworkUDP {
		if options.Remote == "" || options.RemotePort == 0 {
			return ovpn.ServerOptions{}, E.New("`remote` and `remote_port` are required for a UDP static-key server")
		}
		remoteAddress = net.JoinHostPort(options.Remote, strconv.Itoa(int(options.RemotePort)))
	} else if options.Remote != "" || options.RemotePort != 0 {
		return ovpn.ServerOptions{}, E.New("`remote` and `remote_port` are only used by a UDP static-key server")
	}
	topology := options.Topology
	if topology == "" {
		topology = "p2p"
	}
	return ovpn.ServerOptions{
		Mode:         ovpn.ModeStaticKey,
		StaticKey:    staticKey,
		KeyDirection: keyDirection,
		Transport: ovpn.ServerTransportOptions{
			Protocol:      protocol,
			RemoteAddress: remoteAddress,
		},
		Resources: ovpn.ServerResourceOptions{MaxClients: options.MaxClients},
		DataChannel: ovpn.ServerDataChannelOptions{
			MTU:              options.MTU,
			MSSFix:           options.MSSFix,
			MSSFixDisabled:   options.MSSFixDisabled,
			MSSFixMode:       options.MSSFixMode,
			Cipher:           options.Cipher,
			Auth:             options.Auth,
			ReplayWindow:     options.ReplayWindow,
			ReplayWindowTime: time.Duration(options.ReplayWindowTime),
			PacketHeadroom:   ovpntransport.PacketHeadroom,
		},
		Timing: ovpn.ServerTimingOptions{
			PingInterval: time.Duration(options.PingInterval),
			PingRestart:  time.Duration(options.PingRestart),
		},
		Tunnel: ovpn.ServerTunnelOptions{
			Topology:       topology,
			LocalAddress:   slices.Clone(options.Address),
			VPNGateway:     vpnGateway,
			VPNGatewayIPv6: vpnGatewayIPv6,
		},
	}, nil
}

func buildServerTLSOptions(options option.OpenVPNInboundTLSOptions) (ovpn.ServerTLSOptions, int, error) {
	switch options.VerifyClientCertificate {
	case "", "require", "optional", "none":
	default:
		return ovpn.ServerTLSOptions{}, 0, E.New("invalid client certificate policy ", options.VerifyClientCertificate, ", allowed values: require, optional, none")
	}
	certificate, err := requiredMaterialSource("tls.certificate", options.Certificate, options.CertificatePath)
	if err != nil {
		return ovpn.ServerTLSOptions{}, 0, err
	}
	key, err := requiredMaterialSource("tls.key", options.Key, options.KeyPath)
	if err != nil {
		return ovpn.ServerTLSOptions{}, 0, err
	}
	certificateAuthority, err := materialSource("tls.client_certificate", options.ClientCertificate, options.ClientCertificatePath)
	if err != nil {
		return ovpn.ServerTLSOptions{}, 0, err
	}
	remoteCertificateTLS := options.RemoteCertificateTLS
	switch remoteCertificateTLS {
	case "", "server", "client", "none":
	default:
		return ovpn.ServerTLSOptions{}, 0, E.New("invalid `tls.remote_certificate_tls`: ", remoteCertificateTLS)
	}
	if options.RemoteCertificateEKU != "" && remoteCertificateTLS != "" {
		return ovpn.ServerTLSOptions{}, 0, E.New("`tls.remote_certificate_eku` is conflict with `tls.remote_certificate_tls`")
	}
	if remoteCertificateTLS == "" && options.RemoteCertificateEKU == "" {
		remoteCertificateTLS = "client"
	} else if remoteCertificateTLS == "none" {
		remoteCertificateTLS = ""
	}
	clientNameType := options.ClientNameType
	if options.ClientName != "" && clientNameType == "" {
		clientNameType = "name"
	}
	tlsOptions := ovpn.ServerTLSOptions{
		CertificateAuthority:    certificateAuthority,
		Certificate:             certificate,
		Key:                     key,
		VerifyClientCertificate: options.VerifyClientCertificate,
		VerifyX509Name:          options.ClientName,
		VerifyX509Type:          clientNameType,
		PeerFingerprint:         options.PeerFingerprint,
		CRLVerify:               options.CRLPath,
		RemoteCertificateKU:     options.RemoteCertificateKU,
		RemoteCertificateEKU:    options.RemoteCertificateEKU,
		RemoteCertificateTLS:    remoteCertificateTLS,
		NSCertificateType:       options.NSCertificateType,
		CertificateProfile:      options.CertificateProfile,
		VersionMin:              options.VersionMin,
		VersionMax:              options.VersionMax,
		Cipher:                  options.Cipher,
		Groups:                  options.Groups,
	}
	keyDirection := -1
	controlWrap := options.ControlWrap
	if controlWrap != nil && (controlWrap.Type != "" || len(controlWrap.Key) > 0 || controlWrap.KeyPath != "" || controlWrap.Direction != "" || controlWrap.ForceCookie) {
		wrapKey, wrapErr := requiredMaterialSource("tls.control_wrap.key", controlWrap.Key, controlWrap.KeyPath)
		if wrapErr != nil {
			return ovpn.ServerTLSOptions{}, 0, wrapErr
		}
		switch controlWrap.Type {
		case "tls_auth":
			if controlWrap.ForceCookie {
				return ovpn.ServerTLSOptions{}, 0, E.New("`tls.control_wrap.force_cookie` is only supported by `tls_crypt_v2`")
			}
			keyDirection, err = keyDirectionValue(controlWrap.Direction)
			if err != nil {
				return ovpn.ServerTLSOptions{}, 0, err
			}
			tlsOptions.Auth = wrapKey
		case "tls_crypt", "tls_crypt_v2":
			if controlWrap.Direction != "" {
				return ovpn.ServerTLSOptions{}, 0, E.New("`tls.control_wrap.direction` is only supported by `tls_auth`")
			}
			if controlWrap.Type == "tls_crypt" {
				if controlWrap.ForceCookie {
					return ovpn.ServerTLSOptions{}, 0, E.New("`tls.control_wrap.force_cookie` is only supported by `tls_crypt_v2`")
				}
				tlsOptions.Crypt = wrapKey
			} else {
				tlsOptions.CryptV2 = wrapKey
				tlsOptions.CryptV2ForceCookie = controlWrap.ForceCookie
			}
		case "":
			return ovpn.ServerTLSOptions{}, 0, E.New("missing control wrap type")
		default:
			return ovpn.ServerTLSOptions{}, 0, E.New("unknown control wrap type: ", controlWrap.Type)
		}
	}
	return tlsOptions, keyDirection, nil
}

func applyServerPushOptions(serverOptions *ovpn.ServerOptions, options option.OpenVPNServerEndpointOptions) error {
	topology := options.Topology
	if topology == "" {
		topology = "subnet"
	}
	localAddresses := make([]netip.Prefix, 0, len(options.Address))
	for _, prefix := range options.Address {
		if !prefix.IsValid() {
			continue
		}
		if prefix.Addr().Is4() {
			localAddresses = append(localAddresses, netip.PrefixFrom(prefix.Addr(), 32))
		} else {
			localAddresses = append(localAddresses, netip.PrefixFrom(prefix.Addr(), 128))
		}
	}
	serverOptions.Tunnel = ovpn.ServerTunnelOptions{
		AddressPools: slices.Clone(options.Address),
		Topology:     topology,
		LocalAddress: localAddresses,
	}
	if options.Push == nil {
		return nil
	}
	serverOptions.Push.Routes = slices.Clone(options.Push.Routes)
	serverOptions.Push.DNS = slices.Clone(options.Push.DNS)
	serverOptions.Push.SearchDomains = slices.Clone(options.Push.SearchDomains)
	serverOptions.Push.DHCPOptions = slices.Clone(options.Push.DHCPOptions)
	for serverIndex, server := range options.Push.DNSServers {
		addresses := make([]netip.AddrPort, 0, len(server.Addresses))
		for addressIndex, addressValue := range server.Addresses {
			address, err := netip.ParseAddr(addressValue)
			if err == nil {
				addresses = append(addresses, netip.AddrPortFrom(address, 0))
				continue
			}
			addressPort, addressPortErr := netip.ParseAddrPort(addressValue)
			if addressPortErr != nil || addressPort.Port() == 0 {
				return E.New("invalid push.dns_servers[", serverIndex, "].addresses[", addressIndex, "]: ", addressValue)
			}
			addresses = append(addresses, addressPort)
		}
		serverOptions.Push.DNSServers = append(serverOptions.Push.DNSServers, ovpn.TunnelDNSServer{
			Priority:       server.Priority,
			Addresses:      addresses,
			ResolveDomains: slices.Clone(server.ResolveDomains),
			DNSSEC:         server.DNSSEC,
			Transport:      server.Transport,
			SNI:            server.SNI,
		})
	}
	serverOptions.Push.BlockOutsideDNS = options.Push.BlockOutsideDNS
	serverOptions.Push.PingInterval = time.Duration(options.Push.PingInterval)
	serverOptions.Push.PingRestart = time.Duration(options.Push.PingRestart)
	if options.Push.RedirectGateway {
		serverOptions.Push.RedirectGateway = true
		if len(options.Push.RedirectGatewayFlags) > 0 {
			serverOptions.Push.RedirectGatewayFlags = slices.Clone(options.Push.RedirectGatewayFlags)
		} else {
			serverOptions.Push.RedirectGatewayFlags = []string{"def1"}
		}
	}
	return nil
}

func (s *ServerEndpoint) readLoop() {
	defer close(s.readLoopDone)
	for {
		serverPacketBuffers, err := s.server.ReadDataPackets(s.loopContext)
		if err != nil {
			if E.IsClosedOrCanceled(err) || s.loopContext.Err() != nil {
				return
			}
			s.logger.Error(E.Cause(err, "server terminated"))
			return
		}
		packetBuffers := make([]*buf.Buffer, len(serverPacketBuffers))
		for i, packetBuffer := range serverPacketBuffers {
			packetBuffers[i] = packetBuffer.Buffer
		}
		err = s.device.WriteInboundBuffers(packetBuffers)
		buf.ReleaseMulti(packetBuffers)
		if err != nil {
			s.logger.Error(E.Cause(err, "write packet to device"))
			return
		}
	}
}

func (s *ServerEndpoint) Close() error {
	s.started.Store(false)
	s.cancelLoop()
	var serverErr error
	if s.server != nil {
		serverErr = s.server.Close()
	}
	if s.readLoopDone != nil {
		<-s.readLoopDone
	}
	var deviceErr error
	if s.device != nil {
		deviceErr = s.device.Close()
	}
	var listenerErr error
	if s.listener != nil {
		listenerErr = s.listener.Close()
	}
	return E.Errors(serverErr, deviceErr, listenerErr)
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
	return judgeOpenVPNFlow(s.router, s.Tag(), s.Type(), s.localAddresses, network, source, destination, firstPacket)
}

func (s *ServerEndpoint) NewDNSPacket(payload []byte, source M.Socksaddr, destination M.Socksaddr, writer N.PacketWriter) {
	s.newDNSPacket(log.ContextWithNewID(s.ctx), s, payload, source, destination, writer)
}

func (s *ServerEndpoint) WritePackets(packets [][]byte) error {
	if !s.started.Load() {
		return E.New("endpoint is not ready yet")
	}
	packetBuffers := make([]*buf.Buffer, len(packets))
	for i, packet := range packets {
		packetBuffers[i] = buf.As(packet)
	}
	routeMisses, err := s.server.WriteDataPacketBuffersByDestination(packetBuffers)
	if len(routeMisses) > 0 {
		s.writeRouteMisses(routeMisses)
	}
	return err
}

func (s *ServerEndpoint) writePacketBuffersByDestination(packetBuffers []*buf.Buffer) error {
	routeMisses, err := s.server.WriteDataPacketBuffersByDestination(packetBuffers)
	if len(routeMisses) > 0 {
		s.writeRouteMisses(routeMisses)
	}
	return err
}

func (s *ServerEndpoint) writeRouteMisses(routeMisses []*ovpn.RouteMissError) {
	returnPath, headroom := s.device.ReturnPath()
	if returnPath == nil {
		return
	}
	inet4Address, inet6Address := s.PortAddresses()
	replies := make([][]byte, 0, len(routeMisses))
	for _, routeMiss := range routeMisses {
		sourceAddress := packetSourceAddress(routeMiss.Packet, inet4Address, inet6Address)
		reply, built := tun.BuildUnreachable(routeMiss.Packet, sourceAddress, headroom)
		if built {
			replies = append(replies, reply)
		}
	}
	if len(replies) > 0 {
		returnPath.ReturnPackets(replies)
	}
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
		return N.ListenSerial(ctx, s.device, destination, destinationAddresses)
	}
	packetConn, err := s.device.ListenPacket(ctx, destination)
	if err != nil {
		return nil, netip.Addr{}, err
	}
	if destination.IsIP() {
		return packetConn, destination.Addr, nil
	}
	return packetConn, netip.Addr{}, nil
}

func (s *ServerEndpoint) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	packetConn, _, err := s.ListenPacketWithDestination(ctx, destination)
	return packetConn, err
}
