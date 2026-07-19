package openvpn

import (
	"context"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/endpoint"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	ovpntransport "github.com/sagernet/sing-box/transport/openvpn"
	ovpn "github.com/sagernet/sing-openvpn"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing-tun/gtcpip/header"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"

	"go4.org/netipx"
)

var (
	_ adapter.OutboundWithPreferredRoutes = (*ClientEndpoint)(nil)
	_ adapter.FlowOutbound                = (*ClientEndpoint)(nil)
	_ adapter.InterfaceUpdateListener     = (*ClientEndpoint)(nil)
	_ dialer.PacketDialerWithDestination  = (*ClientEndpoint)(nil)
	_ tun.Port                            = (*ClientEndpoint)(nil)
)

type ClientEndpoint struct {
	endpointBase
	ctx               context.Context
	loopContext       context.Context
	cancelLoop        context.CancelFunc
	dnsRouter         adapter.DNSRouter
	outboundDialer    N.Dialer
	queryOptions      adapter.DNSQueryOptions
	client            *ovpn.Client
	device            ovpntransport.Device
	stateAccess       sync.Mutex
	state             atomic.Pointer[clientState]
	deviceStarted     bool
	readLoopDone      chan struct{}
	statusAccess      sync.Mutex
	statusUpdated     chan struct{}
	terminalError     string
	challengeLoopDone chan struct{}
}

type clientState struct {
	started          bool
	tunnelConfigured bool
	localAddresses   []netip.Prefix
	routeSet         *netipx.IPSet
	blockIPv6        bool
	tunnelInfo       adapter.OpenVPNTunnelInfo
}

func NewClientEndpoint(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.OpenVPNClientEndpointOptions) (adapter.Endpoint, error) {
	loopContext, cancelLoop := context.WithCancel(ctx)
	clientEndpoint := &ClientEndpoint{
		endpointBase: endpointBase{
			Adapter: endpoint.NewAdapterWithDialerOptions(C.TypeOpenVPNClient, tag, []string{N.NetworkTCP, N.NetworkUDP, N.NetworkICMP}, options.DialerOptions),
			router:  router,
			logger:  logger,
		},
		ctx:           ctx,
		loopContext:   loopContext,
		cancelLoop:    cancelLoop,
		dnsRouter:     service.FromContext[adapter.DNSRouter](ctx),
		statusUpdated: make(chan struct{}),
	}
	success := false
	defer func() {
		if success {
			return
		}
		if clientEndpoint.device != nil {
			_ = clientEndpoint.device.Close()
		}
		cancelLoop()
	}()
	clientOptions, err := clientEndpoint.buildClientOptions(options)
	if err != nil {
		return nil, err
	}
	clientEndpoint.state.Store(&clientState{localAddresses: clientOptions.Tunnel.LocalAddress})
	outboundDialer, err := dialer.NewWithOptions(dialer.Options{
		Context:          ctx,
		Options:          options.DialerOptions,
		RemoteIsDomain:   openVPNClientRemoteIsDomain(options),
		ResolverOnDetour: true,
		NewDialer:        true,
	})
	if err != nil {
		return nil, err
	}
	var queryOptions adapter.DNSQueryOptions
	resolveDialer, isResolveDialer := outboundDialer.(dialer.ResolveDialer)
	if isResolveDialer {
		queryOptions = resolveDialer.QueryOptions()
	}
	clientEndpoint.outboundDialer = outboundDialer
	clientEndpoint.queryOptions = queryOptions
	udpTimeout := C.UDPTimeout
	if options.UDPTimeout != 0 {
		udpTimeout = time.Duration(options.UDPTimeout)
	}
	device, err := ovpntransport.NewDevice(ovpntransport.DeviceOptions{
		Context:         ctx,
		Logger:          logger,
		System:          options.System,
		Handler:         clientEndpoint,
		UDPTimeout:      udpTimeout,
		ICMPTimeout:     C.ICMPTimeout,
		UDPMapping:      tun.NATMapping(options.UDPMapping),
		UDPFiltering:    tun.NATFiltering(options.UDPFiltering),
		UDPNATMax:       options.UDPNATMax,
		InterfaceFinder: service.FromContext[adapter.NetworkManager](ctx).InterfaceFinder(),
		Name:            options.Name,
		MTU:             options.MTU,
		Configuration: ovpntransport.Configuration{
			MTU:     options.MTU,
			Address: clientOptions.Tunnel.LocalAddress,
		},
	})
	if err != nil {
		return nil, err
	}
	clientEndpoint.device = device
	device.SetPacketWriter(clientEndpoint.writePacketBuffers)
	client, err := ovpn.NewClient(clientOptions)
	if err != nil {
		return nil, err
	}
	clientEndpoint.client = client
	success = true
	return clientEndpoint, nil
}

func (c *ClientEndpoint) buildClientOptions(options option.OpenVPNClientEndpointOptions) (ovpn.ClientOptions, error) {
	if options.TLS == nil {
		return ovpn.ClientOptions{}, E.New("missing `tls` options")
	}
	if options.Server != "" && len(options.Servers) > 0 {
		return ovpn.ClientOptions{}, E.New("`server` is conflict with `servers`")
	}
	if options.Server == "" && len(options.Servers) == 0 {
		return ovpn.ClientOptions{}, E.New("missing `server` or `servers`")
	}
	certificateAuthority, err := materialSource("tls.certificate", options.TLS.Certificate, options.TLS.CertificatePath)
	if err != nil {
		return ovpn.ClientOptions{}, err
	}
	clientCertificate, err := materialSource("tls.client_certificate", options.TLS.ClientCertificate, options.TLS.ClientCertificatePath)
	if err != nil {
		return ovpn.ClientOptions{}, err
	}
	clientKey, err := materialSource("tls.client_key", options.TLS.ClientKey, options.TLS.ClientKeyPath)
	if err != nil {
		return ovpn.ClientOptions{}, err
	}
	keyDirection := -1
	var controlAuth ovpn.Material
	var controlCrypt ovpn.Material
	var controlCryptV2 ovpn.Material
	controlWrap := options.TLS.ControlWrap
	if controlWrap != nil && (controlWrap.Type != "" || len(controlWrap.Key) > 0 || controlWrap.KeyPath != "" || controlWrap.Direction != "") {
		controlKey, controlErr := requiredMaterialSource("tls.control_wrap.key", controlWrap.Key, controlWrap.KeyPath)
		if controlErr != nil {
			return ovpn.ClientOptions{}, controlErr
		}
		switch controlWrap.Type {
		case "tls_auth":
			keyDirection, err = keyDirectionValue(controlWrap.Direction)
			if err != nil {
				return ovpn.ClientOptions{}, err
			}
			controlAuth = controlKey
		case "tls_crypt":
			if controlWrap.Direction != "" {
				return ovpn.ClientOptions{}, E.New("`tls.control_wrap.direction` is only supported by `tls_auth`")
			}
			controlCrypt = controlKey
		case "tls_crypt_v2":
			if controlWrap.Direction != "" {
				return ovpn.ClientOptions{}, E.New("`tls.control_wrap.direction` is only supported by `tls_auth`")
			}
			controlCryptV2 = controlKey
		case "":
			return ovpn.ClientOptions{}, E.New("missing OpenVPN control wrap type")
		default:
			return ovpn.ClientOptions{}, E.New("unknown OpenVPN control wrap type: ", controlWrap.Type)
		}
	}
	protocol := options.Network
	if protocol == "" {
		protocol = N.NetworkUDP
	}
	var remotes []ovpn.Remote
	if options.Server != "" {
		remotes = append(remotes, ovpn.Remote{
			Host:     options.Server,
			Port:     options.ServerPort,
			Protocol: protocol,
		})
	} else {
		remotes = make([]ovpn.Remote, 0, len(options.Servers))
		for _, remoteOptions := range options.Servers {
			remoteProtocol := remoteOptions.Network
			if remoteProtocol == "" {
				remoteProtocol = protocol
			}
			remotes = append(remotes, ovpn.Remote{
				Host:     remoteOptions.Server,
				Port:     remoteOptions.ServerPort,
				Protocol: remoteProtocol,
			})
		}
	}
	pullFilters := common.Map(options.PullFilters, func(filterOptions option.OpenVPNPullFilterOptions) ovpn.PullFilter {
		return ovpn.PullFilter{
			Action: filterOptions.Action,
			Text:   filterOptions.Text,
		}
	})
	tunnelRoutes := common.Map(options.Routes, func(route netip.Prefix) ovpn.TunnelRoute {
		return ovpn.TunnelRoute{Prefix: route}
	})
	clientTLSOptions := ovpn.ClientTLSOptions{
		CertificateAuthority: certificateAuthority,
		Certificate:          clientCertificate,
		Key:                  clientKey,
		Auth:                 controlAuth,
		Crypt:                controlCrypt,
		CryptV2:              controlCryptV2,
		VerifyX509Type:       options.TLS.ServerNameType,
		PeerFingerprint:      options.TLS.PeerFingerprint,
		CRLVerify:            options.TLS.CRLPath,
		RemoteCertificateKU:  options.TLS.RemoteCertificateKU,
		RemoteCertificateEKU: options.TLS.RemoteCertificateEKU,
		RemoteCertificateTLS: "server",
		VersionMin:           options.TLS.VersionMin,
		VersionMax:           options.TLS.VersionMax,
		Cipher:               options.TLS.Cipher,
		Groups:               options.TLS.Groups,
	}
	if options.TLS.ServerName != "" {
		clientTLSOptions.VerifyX509Name = options.TLS.ServerName
		if options.TLS.ServerNameType == "" {
			clientTLSOptions.VerifyX509Type = "name"
		}
	}
	return ovpn.ClientOptions{
		Context: c.loopContext,
		Mode:    ovpn.ModeTLS,
		Transport: ovpn.ClientTransportOptions{
			Remotes:                     remotes,
			RemoteRandom:                options.RemoteRandom,
			Protocol:                    protocol,
			ExplicitExitNotify:          options.ExplicitExitNotify,
			DialContextWithAddressIndex: c.transportDialContextWithAddressIndex,
		},
		DataChannel: ovpn.ClientDataChannelOptions{
			MTU:              options.MTU,
			MSSFix:           options.MSSFix,
			Fragment:         options.Fragment,
			Ciphers:          options.DataCiphers,
			FallbackCipher:   options.DataCiphersFallback,
			Auth:             options.Auth,
			Compression:      options.Compression,
			CompressionLZO:   options.CompressionLZO,
			AllowCompression: options.AllowCompression,
			PacketHeadroom:   ovpntransport.PacketHeadroom,
		},
		TLS: clientTLSOptions,
		Authentication: ovpn.ClientAuthenticationOptions{
			Username:            options.Username,
			Password:            options.Password,
			AuthRetry:           options.AuthRetry,
			StaticChallenge:     options.StaticChallenge,
			StaticChallengeEcho: options.StaticChallengeEcho,
		},
		Pull: ovpn.ClientPullOptions{
			Enabled:     true,
			Filters:     pullFilters,
			RouteNoPull: options.RouteNoPull,
		},
		Tunnel: ovpn.ClientTunnelOptions{
			DevType:              "tun",
			RedirectGateway:      options.RedirectGateway,
			RedirectGatewayFlags: options.RedirectGatewayFlags,
			RouteMetric:          options.RouteMetric,
			RouteGateway:         options.RouteGateway.Build(netip.Addr{}),
			Routes:               tunnelRoutes,
		},
		Timing: ovpn.ClientTimingOptions{
			RenegotiationInterval: time.Duration(options.RenegotiateInterval),
			PingInterval:          time.Duration(options.PingInterval),
			PingRestart:           time.Duration(options.PingRestart),
		},
		KeyDirection:          keyDirection,
		OnTunnelConfiguration: c.handleTunnelConfiguration,
		Logger:                c.logger,
	}, nil
}

func (c *ClientEndpoint) transportDialContextWithAddressIndex(ctx context.Context, network string, address string, addressIndex int) (net.Conn, error) {
	destination := M.ParseSocksaddr(address)
	if destination.IsDomain() {
		destinationAddresses, lookupErr := c.dnsRouter.Lookup(ctx, destination.Fqdn, c.queryOptions)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if addressIndex < 0 || addressIndex >= len(destinationAddresses) {
			return nil, ovpn.ErrRemoteAddressExhausted
		}
		destination = M.SocksaddrFrom(destinationAddresses[addressIndex], destination.Port)
	} else if addressIndex != 0 {
		return nil, ovpn.ErrRemoteAddressExhausted
	}
	connection, err := c.outboundDialer.DialContext(ctx, network, destination)
	if err != nil {
		return nil, err
	}
	if N.NetworkName(network) == N.NetworkUDP {
		tuneOpenVPNUDPSocket(connection)
	}
	c.stateAccess.Lock()
	c.updateState(func(state *clientState) {
		state.tunnelInfo.Server = address
		state.tunnelInfo.Network = N.NetworkName(network)
	})
	c.stateAccess.Unlock()
	return connection, nil
}

func (c *ClientEndpoint) handleTunnelConfiguration(event ovpn.TunnelConfigurationEvent) error {
	configuration := configurationFromClientEvent(event, c.logger)
	defer c.notifyStatusUpdated()
	c.stateAccess.Lock()
	defer c.stateAccess.Unlock()
	c.updateState(func(state *clientState) {
		state.tunnelConfigured = false
	})
	err := c.device.UpdateConfiguration(configuration)
	if err != nil {
		return E.Cause(err, "update device configuration")
	}
	if !c.deviceStarted {
		err = c.device.Start()
		if err != nil {
			return E.Cause(err, "start device")
		}
		c.deviceStarted = true
	}
	routeSet, err := buildIPSet(configuration.Routes)
	if err != nil {
		return E.Cause(err, "build route set")
	}
	c.updateState(func(state *clientState) {
		state.tunnelConfigured = true
		state.localAddresses = configuration.Address
		state.routeSet = routeSet
		state.blockIPv6 = configuration.BlockIPv6
		state.tunnelInfo.Cipher = event.Configuration.SelectedCipher
		state.tunnelInfo.IPv4 = event.Configuration.LocalIPv4
		state.tunnelInfo.IPv6 = event.Configuration.LocalIPv6
		state.tunnelInfo.DNS = event.Configuration.DNS
		state.tunnelInfo.MTU = configuration.MTU
		if event.Reason == ovpn.TunnelConfigurationEventInitial || state.tunnelInfo.ConnectedSince.IsZero() {
			state.tunnelInfo.ConnectedSince = time.Now()
		}
	})
	return nil
}

func (c *ClientEndpoint) updateState(update func(state *clientState)) {
	newState := *c.state.Load()
	update(&newState)
	c.state.Store(&newState)
}

func (c *ClientEndpoint) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStatePostStart {
		return nil
	}
	err := c.client.Start()
	if err != nil {
		return err
	}
	c.stateAccess.Lock()
	c.updateState(func(state *clientState) {
		state.started = true
	})
	c.readLoopDone = make(chan struct{})
	c.challengeLoopDone = make(chan struct{})
	c.stateAccess.Unlock()
	go c.readLoop()
	go c.watchChallenges()
	return nil
}

func (c *ClientEndpoint) readLoop() {
	defer close(c.readLoopDone)
	for {
		packetBuffers, err := c.client.ReadDataPackets(c.loopContext)
		if err != nil {
			if E.IsClosedOrCanceled(err) || c.loopContext.Err() != nil {
				return
			}
			c.logger.Error(E.Cause(err, "OpenVPN client terminated"))
			c.setTerminalError(err)
			return
		}
		err = c.device.WriteInboundBuffers(packetBuffers)
		buf.ReleaseMulti(packetBuffers)
		if err != nil {
			err = E.Cause(err, "write packet to device")
			c.logger.Error(err)
			c.setTerminalError(err)
			return
		}
	}
}

func (c *ClientEndpoint) Close() error {
	c.stateAccess.Lock()
	c.updateState(func(state *clientState) {
		state.started = false
	})
	readLoopDone := c.readLoopDone
	challengeLoopDone := c.challengeLoopDone
	c.stateAccess.Unlock()
	c.cancelLoop()
	err := E.Errors(c.client.Close(), c.device.Close())
	if readLoopDone != nil {
		<-readLoopDone
	}
	if challengeLoopDone != nil {
		<-challengeLoopDone
	}
	c.notifyStatusUpdated()
	return err
}

func (c *ClientEndpoint) InterfaceUpdated() {
	c.client.RestartSession()
}

func (c *ClientEndpoint) PreMatchFlow(network string, destination netip.Addr) adapter.PreMatchAction {
	return adapter.PreMatchFlow
}

func (c *ClientEndpoint) PortAddresses() (netip.Addr, netip.Addr) {
	return c.device.PortAddresses()
}

func (c *ClientEndpoint) PortMTU() uint32 {
	return c.device.PortMTU()
}

func (c *ClientEndpoint) AttachReturn(returnPath tun.Return) error {
	return c.device.AttachReturn(returnPath)
}

func (c *ClientEndpoint) DetachReturn(returnPath tun.Return) error {
	return c.device.DetachReturn(returnPath)
}

func (c *ClientEndpoint) JudgeFlow(network uint8, source netip.AddrPort, destination netip.AddrPort, firstPacket []byte) tun.FlowVerdict {
	return judgeOpenVPNFlow(c.router, c.Tag(), c.Type(), c.state.Load().localAddresses, network, source, destination, firstPacket)
}

func (c *ClientEndpoint) NewDNSPacket(payload []byte, source M.Socksaddr, destination M.Socksaddr, writer N.PacketWriter) {
	c.newDNSPacket(log.ContextWithNewID(c.ctx), c, payload, source, destination, writer)
}

func (c *ClientEndpoint) ready() bool {
	state := c.state.Load()
	return state.started && state.tunnelConfigured
}

func (c *ClientEndpoint) WritePackets(packets [][]byte) error {
	state := c.state.Load()
	if !state.started || !state.tunnelConfigured {
		return E.New("OpenVPN client is not ready yet")
	}
	if state.blockIPv6 {
		outboundPackets := packets[:0]
		for _, packet := range packets {
			if header.IPVersion(packet) != header.IPv6Version {
				outboundPackets = append(outboundPackets, packet)
			}
		}
		packets = outboundPackets
		if len(packets) == 0 {
			return nil
		}
	}
	packetBuffers := make([]*buf.Buffer, len(packets))
	for i, packet := range packets {
		packetBuffers[i] = buf.As(packet)
	}
	err := c.client.WriteDataPacketBuffers(packetBuffers)
	if E.IsMulti(err, ovpn.ErrDataChannelNotReady) {
		return E.New("OpenVPN client is not ready yet")
	}
	return err
}

func (c *ClientEndpoint) writePacketBuffers(packetBuffers []*buf.Buffer) error {
	state := c.state.Load()
	if !state.started || !state.tunnelConfigured {
		buf.ReleaseMulti(packetBuffers)
		return nil
	}
	if state.blockIPv6 {
		outboundBuffers := packetBuffers[:0]
		for _, packetBuffer := range packetBuffers {
			if header.IPVersion(packetBuffer.Bytes()) == header.IPv6Version {
				packetBuffer.Release()
				continue
			}
			outboundBuffers = append(outboundBuffers, packetBuffer)
		}
		packetBuffers = outboundBuffers
		if len(packetBuffers) == 0 {
			return nil
		}
	}
	err := c.client.WriteDataPacketBuffers(packetBuffers)
	if E.IsMulti(err, ovpn.ErrDataChannelNotReady) {
		return nil
	}
	return err
}

func (c *ClientEndpoint) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	c.newConnection(ctx, c, c.state.Load().localAddresses, conn, source, destination, onClose)
}

func (c *ClientEndpoint) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	c.newPacketConnection(ctx, c, c.state.Load().localAddresses, conn, source, destination, onClose)
}

func (c *ClientEndpoint) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	switch network {
	case N.NetworkTCP:
		c.logger.InfoContext(ctx, "outbound connection to ", destination)
	case N.NetworkUDP:
		c.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	}
	if !c.ready() || !c.client.Ready() {
		return nil, E.New("OpenVPN client is not ready yet")
	}
	if destination.IsDomain() {
		destinationAddresses, err := c.dnsRouter.Lookup(ctx, destination.Fqdn, adapter.DNSQueryOptions{})
		if err != nil {
			return nil, err
		}
		return N.DialSerial(ctx, c.device, network, destination, destinationAddresses)
	}
	if !destination.Addr.IsValid() {
		return nil, E.New("invalid destination: ", destination)
	}
	return c.device.DialContext(ctx, network, destination)
}

func (c *ClientEndpoint) ListenPacketWithDestination(ctx context.Context, destination M.Socksaddr) (net.PacketConn, netip.Addr, error) {
	c.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	if !c.ready() || !c.client.Ready() {
		return nil, netip.Addr{}, E.New("OpenVPN client is not ready yet")
	}
	if destination.IsDomain() {
		destinationAddresses, err := c.dnsRouter.Lookup(ctx, destination.Fqdn, adapter.DNSQueryOptions{})
		if err != nil {
			return nil, netip.Addr{}, err
		}
		return N.ListenSerial(ctx, c.device, destination, destinationAddresses)
	}
	packetConn, err := c.device.ListenPacket(ctx, destination)
	if err != nil {
		return nil, netip.Addr{}, err
	}
	if destination.IsIP() {
		return packetConn, destination.Addr, nil
	}
	return packetConn, netip.Addr{}, nil
}

func (c *ClientEndpoint) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	packetConn, destinationAddress, err := c.ListenPacketWithDestination(ctx, destination)
	if err != nil {
		return nil, err
	}
	if destinationAddress.IsValid() && destination != M.SocksaddrFrom(destinationAddress, destination.Port) {
		return bufio.NewNATPacketConn(bufio.NewPacketConn(packetConn), M.SocksaddrFrom(destinationAddress, destination.Port), destination), nil
	}
	return packetConn, nil
}

func (c *ClientEndpoint) PreferredDomain(metadata *adapter.InboundContext, domain string) bool {
	return false
}

func (c *ClientEndpoint) PreferredAddress(metadata *adapter.InboundContext, address netip.Addr) bool {
	state := c.state.Load()
	if !state.started || !state.tunnelConfigured || state.routeSet == nil || !c.client.Ready() {
		return false
	}
	return state.routeSet.Contains(address)
}
