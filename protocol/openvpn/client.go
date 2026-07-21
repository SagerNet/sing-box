package openvpn

import (
	"context"
	"net"
	"net/netip"
	"slices"
	"strings"
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
	dnsTransport      *DNSTransport
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
	configuration    ovpntransport.Configuration
	preferredDomains []string
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
	mode := options.Mode
	if mode == "" {
		mode = ovpn.ModeTLS
	}
	switch mode {
	case ovpn.ModeTLS, ovpn.ModeStaticKey:
	default:
		return ovpn.ClientOptions{}, E.New("unsupported mode: ", mode, " (expected \"tls\" or \"static_key\")")
	}
	if options.Server != "" && len(options.Servers) > 0 {
		return ovpn.ClientOptions{}, E.New("`server` is conflict with `servers`")
	}
	if options.Server == "" && len(options.Servers) == 0 {
		return ovpn.ClientOptions{}, E.New("missing `server` or `servers`")
	}
	protocol, remotes := buildClientRemoteOptions(options)
	tunnelOptions, err := buildClientTunnelOptions(options, mode == ovpn.ModeStaticKey)
	if err != nil {
		return ovpn.ClientOptions{}, err
	}
	if mode == ovpn.ModeStaticKey {
		return c.buildStaticKeyClientOptions(options, protocol, remotes, tunnelOptions)
	}
	if options.TLS == nil {
		return ovpn.ClientOptions{}, E.New("missing `tls` options")
	}
	if len(options.StaticKey) > 0 || options.StaticKeyPath != "" {
		return ovpn.ClientOptions{}, E.New("`static_key` and `static_key_path` are only supported in `static_key` mode")
	}
	if options.KeyDirection != "" {
		return ovpn.ClientOptions{}, E.New("`key_direction` is only supported in `static_key` mode; use `tls.control_wrap.direction` for `tls_auth`")
	}
	if options.Cipher != "" {
		return ovpn.ClientOptions{}, E.New("`cipher` is only supported in `static_key` mode; use `data_ciphers` or `data_ciphers_fallback` in TLS mode")
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
			return ovpn.ClientOptions{}, E.New("missing control wrap type")
		default:
			return ovpn.ClientOptions{}, E.New("unknown control wrap type: ", controlWrap.Type)
		}
	}
	pullFilters := common.Map(options.PullFilters, func(filterOptions option.OpenVPNPullFilterOptions) ovpn.PullFilter {
		return ovpn.PullFilter{
			Action: filterOptions.Action,
			Text:   filterOptions.Text,
		}
	})
	remoteCertificateTLS := options.TLS.RemoteCertificateTLS
	switch remoteCertificateTLS {
	case "", "server", "client", "none":
	default:
		return ovpn.ClientOptions{}, E.New("invalid `tls.remote_certificate_tls`: ", remoteCertificateTLS)
	}
	if options.TLS.RemoteCertificateEKU != "" && remoteCertificateTLS != "" {
		return ovpn.ClientOptions{}, E.New("`tls.remote_certificate_eku` is conflict with `tls.remote_certificate_tls`")
	}
	if remoteCertificateTLS == "" && options.TLS.RemoteCertificateEKU == "" {
		remoteCertificateTLS = "server"
	} else if remoteCertificateTLS == "none" {
		remoteCertificateTLS = ""
	}
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
		RemoteCertificateTLS: remoteCertificateTLS,
		NSCertificateType:    options.TLS.NSCertificateType,
		VersionMin:           options.TLS.VersionMin,
		VersionMax:           options.TLS.VersionMax,
		CertificateProfile:   options.TLS.CertificateProfile,
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
		Mode:    mode,
		Transport: ovpn.ClientTransportOptions{
			Remotes:                     remotes,
			RemoteRandom:                options.RemoteRandom,
			Protocol:                    protocol,
			ExplicitExitNotify:          options.ExplicitExitNotify,
			DialContextWithAddressIndex: c.transportDialContextWithAddressIndex,
		},
		DataChannel: buildClientDataChannelOptions(options),
		TLS:         clientTLSOptions,
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
		Tunnel:                tunnelOptions,
		Timing:                buildClientTimingOptions(options),
		KeyDirection:          keyDirection,
		OnTunnelConfiguration: c.handleTunnelConfiguration,
		Logger:                c.logger,
	}, nil
}

func (c *ClientEndpoint) buildStaticKeyClientOptions(options option.OpenVPNClientEndpointOptions, protocol string, remotes []ovpn.Remote, tunnelOptions ovpn.ClientTunnelOptions) (ovpn.ClientOptions, error) {
	if options.TLS != nil {
		return ovpn.ClientOptions{}, E.New("`tls` options are not supported in `static_key` mode")
	}
	if options.Username != "" || options.Password != "" || (options.AuthRetry != "" && options.AuthRetry != "none") || options.StaticChallenge != "" || options.StaticChallengeEcho {
		return ovpn.ClientOptions{}, E.New("username/password authentication is not supported in `static_key` mode")
	}
	if options.RouteNoPull || len(options.PullFilters) > 0 {
		return ovpn.ClientOptions{}, E.New("pull options are not supported in `static_key` mode")
	}
	if options.RenegotiateInterval != 0 || options.RenegotiateDisabled || options.RenegotiateBytes != 0 || options.RenegotiatePackets != 0 || options.TLSTimeout != 0 || options.HandshakeWindow != 0 {
		return ovpn.ClientOptions{}, E.New("TLS timing and renegotiation options are not supported in `static_key` mode")
	}
	if len(options.DataCiphers) > 0 || options.DataCiphersFallback != "" {
		return ovpn.ClientOptions{}, E.New("`data_ciphers` and `data_ciphers_fallback` are not supported in `static_key` mode; use `cipher`")
	}
	staticKey, err := requiredMaterialSource("static_key", options.StaticKey, options.StaticKeyPath)
	if err != nil {
		return ovpn.ClientOptions{}, err
	}
	keyDirection, err := keyDirectionValue(options.KeyDirection)
	if err != nil {
		return ovpn.ClientOptions{}, err
	}
	return ovpn.ClientOptions{
		Context: c.loopContext,
		Mode:    ovpn.ModeStaticKey,
		Transport: ovpn.ClientTransportOptions{
			Remotes:                     remotes,
			RemoteRandom:                options.RemoteRandom,
			Protocol:                    protocol,
			ExplicitExitNotify:          options.ExplicitExitNotify,
			DialContextWithAddressIndex: c.transportDialContextWithAddressIndex,
		},
		DataChannel:           buildClientDataChannelOptions(options),
		Tunnel:                tunnelOptions,
		Timing:                buildClientTimingOptions(options),
		StaticKey:             staticKey,
		KeyDirection:          keyDirection,
		OnTunnelConfiguration: c.handleTunnelConfiguration,
		Logger:                c.logger,
	}, nil
}

func buildClientRemoteOptions(options option.OpenVPNClientEndpointOptions) (string, []ovpn.Remote) {
	protocol := options.Network
	if protocol == "" {
		protocol = N.NetworkUDP
	}
	if options.Server != "" {
		return protocol, []ovpn.Remote{{
			Host:     options.Server,
			Port:     options.ServerPort,
			Protocol: protocol,
		}}
	}
	remotes := make([]ovpn.Remote, 0, len(options.Servers))
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
	return protocol, remotes
}

func buildClientDataChannelOptions(options option.OpenVPNClientEndpointOptions) ovpn.ClientDataChannelOptions {
	return ovpn.ClientDataChannelOptions{
		MTU:              options.MTU,
		MSSFix:           options.MSSFix,
		MSSFixDisabled:   options.MSSFixDisabled,
		MSSFixMode:       options.MSSFixMode,
		Fragment:         options.Fragment,
		Cipher:           options.Cipher,
		Ciphers:          options.DataCiphers,
		FallbackCipher:   options.DataCiphersFallback,
		Auth:             options.Auth,
		Compression:      options.Compression,
		CompressionLZO:   options.CompressionLZO,
		AllowCompression: options.AllowCompression,
		ReplayWindow:     options.ReplayWindow,
		ReplayWindowTime: time.Duration(options.ReplayWindowTime),
		PacketHeadroom:   ovpntransport.PacketHeadroom,
	}
}

func buildClientTunnelOptions(options option.OpenVPNClientEndpointOptions, requirePeerAddress bool) (ovpn.ClientTunnelOptions, error) {
	vpnGateway := netip.Addr(options.PeerAddress)
	if vpnGateway.IsValid() && !vpnGateway.Is4() {
		return ovpn.ClientTunnelOptions{}, E.New("`peer_address` must be an IPv4 address")
	}
	vpnGatewayIPv6 := netip.Addr(options.PeerAddressIPv6)
	if vpnGatewayIPv6.IsValid() && !vpnGatewayIPv6.Is6() {
		return ovpn.ClientTunnelOptions{}, E.New("`peer_address_ipv6` must be an IPv6 address")
	}
	var hasIPv4 bool
	var hasIPv6 bool
	for addressIndex, address := range options.Address {
		if !address.IsValid() {
			return ovpn.ClientTunnelOptions{}, E.New("`address[", addressIndex, "]` is invalid")
		}
		if address.Addr().Is4() {
			hasIPv4 = true
		} else {
			hasIPv6 = true
		}
	}
	if requirePeerAddress {
		if len(options.Address) == 0 {
			return ovpn.ClientTunnelOptions{}, E.New("missing `address` in `static_key` mode")
		}
		if hasIPv4 && !vpnGateway.IsValid() {
			return ovpn.ClientTunnelOptions{}, E.New("missing `peer_address` for the IPv4 tunnel address in `static_key` mode")
		}
		if hasIPv6 && !vpnGatewayIPv6.IsValid() {
			return ovpn.ClientTunnelOptions{}, E.New("missing `peer_address_ipv6` for the IPv6 tunnel address in `static_key` mode")
		}
		if vpnGateway.IsValid() && !hasIPv4 {
			return ovpn.ClientTunnelOptions{}, E.New("`peer_address` requires an IPv4 tunnel `address` in `static_key` mode")
		}
		if vpnGatewayIPv6.IsValid() && !hasIPv6 {
			return ovpn.ClientTunnelOptions{}, E.New("`peer_address_ipv6` requires an IPv6 tunnel `address` in `static_key` mode")
		}
	}
	tunnelRoutes := common.Map(options.Routes, func(route netip.Prefix) ovpn.TunnelRoute {
		return ovpn.TunnelRoute{Prefix: route}
	})
	return ovpn.ClientTunnelOptions{
		DevType:              "tun",
		Topology:             options.Topology,
		RedirectGateway:      options.RedirectGateway,
		RedirectGatewayFlags: options.RedirectGatewayFlags,
		RedirectPrivate:      options.RedirectPrivate,
		BlockIPv6:            options.BlockIPv6,
		RouteMetric:          options.RouteMetric,
		RouteGateway:         options.RouteGateway.Build(netip.Addr{}),
		Routes:               tunnelRoutes,
		LocalAddress:         options.Address,
		VPNGateway:           vpnGateway,
		VPNGatewayIPv6:       vpnGatewayIPv6,
	}, nil
}

func buildClientTimingOptions(options option.OpenVPNClientEndpointOptions) ovpn.ClientTimingOptions {
	return ovpn.ClientTimingOptions{
		RenegotiationInterval: time.Duration(options.RenegotiateInterval),
		RenegotiationDisabled: options.RenegotiateDisabled,
		RenegotiationBytes:    options.RenegotiateBytes,
		RenegotiationPackets:  options.RenegotiatePackets,
		PingInterval:          time.Duration(options.PingInterval),
		PingRestart:           time.Duration(options.PingRestart),
		PingRestartDisabled:   options.PingRestartDisabled,
		TLSTimeout:            time.Duration(options.TLSTimeout),
		HandWindow:            time.Duration(options.HandshakeWindow),
	}
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
	defer c.notifyStatusUpdated()
	c.stateAccess.Lock()
	configuration := configurationFromClientEvent(event, c.logger)
	c.updateState(func(state *clientState) {
		state.tunnelConfigured = false
	})
	deviceConfiguration := ovpntransport.Configuration{
		MTU:       configuration.MTU,
		Address:   configuration.Address,
		BlockIPv6: configuration.BlockIPv6,
	}
	err := c.device.UpdateConfiguration(deviceConfiguration)
	if err != nil {
		c.stateAccess.Unlock()
		return E.Cause(err, "update device configuration")
	}
	if !c.deviceStarted {
		err = c.device.Start()
		if err != nil {
			c.stateAccess.Unlock()
			return E.Cause(err, "start device")
		}
		c.deviceStarted = true
	}
	routeSet, err := buildIPSet(configuration.Routes, configuration.ExcludedRoutes)
	if err != nil {
		c.stateAccess.Unlock()
		return E.Cause(err, "build route set")
	}
	preferredDomains := slices.Clone(configuration.DNSRoutes)
	preferredDomains = append(preferredDomains, configuration.SearchDomains...)
	if len(configuration.DNSServers) > 0 {
		servers := slices.Clone(configuration.DNSServers)
		slices.SortFunc(servers, func(left ovpntransport.DNSServer, right ovpntransport.DNSServer) int {
			return left.Priority - right.Priority
		})
		preferredDomains = append(preferredDomains, servers[0].ResolveDomains...)
	}
	c.updateState(func(state *clientState) {
		state.tunnelConfigured = true
		state.localAddresses = configuration.Address
		state.routeSet = routeSet
		state.blockIPv6 = configuration.BlockIPv6
		state.configuration = configuration
		state.preferredDomains = preferredDomains
		state.tunnelInfo.Cipher = event.Configuration.SelectedCipher
		state.tunnelInfo.IPv4 = event.Configuration.LocalIPv4
		state.tunnelInfo.IPv6 = event.Configuration.LocalIPv6
		state.tunnelInfo.DNS = event.Configuration.DNS
		state.tunnelInfo.MTU = configuration.MTU
		if event.Reason == ovpn.TunnelConfigurationEventInitial || state.tunnelInfo.ConnectedSince.IsZero() {
			state.tunnelInfo.ConnectedSince = time.Now()
		}
	})
	dnsTransport := c.dnsTransport
	c.stateAccess.Unlock()
	if dnsTransport != nil {
		dnsTransport.onReconfiguration(configuration)
	}
	return nil
}

func (c *ClientEndpoint) updateState(update func(state *clientState)) {
	newState := *c.state.Load()
	update(&newState)
	c.state.Store(&newState)
}

func (c *ClientEndpoint) installDNSTransport(dnsTransport *DNSTransport) error {
	c.stateAccess.Lock()
	defer c.stateAccess.Unlock()
	if c.dnsTransport != nil && c.dnsTransport != dnsTransport && c.dnsTransport.Tag() != dnsTransport.Tag() {
		return E.New("only one DNS server is allowed for an endpoint")
	}
	err := dnsTransport.updateResolvers(c.state.Load().configuration)
	if err != nil {
		return err
	}
	c.dnsTransport = dnsTransport
	return nil
}

func (c *ClientEndpoint) uninstallDNSTransport(dnsTransport *DNSTransport) {
	c.stateAccess.Lock()
	if c.dnsTransport == dnsTransport {
		c.dnsTransport = nil
	}
	c.stateAccess.Unlock()
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
			c.logger.Error(E.Cause(err, "client terminated"))
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
		return E.New("endpoint is not ready yet")
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
		return E.New("endpoint is not ready yet")
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
		return nil, E.New("endpoint is not ready yet")
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
		return nil, netip.Addr{}, E.New("endpoint is not ready yet")
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
	state := c.state.Load()
	if !state.started || !state.tunnelConfigured || !c.client.Ready() {
		return false
	}
	for _, preferredDomain := range state.preferredDomains {
		if openVPNDomainMatches(preferredDomain, domain) {
			return true
		}
	}
	return false
}

func (c *ClientEndpoint) PreferredAddress(metadata *adapter.InboundContext, address netip.Addr) bool {
	state := c.state.Load()
	if !state.started || !state.tunnelConfigured || state.routeSet == nil || !c.client.Ready() {
		return false
	}
	return state.routeSet.Contains(address)
}

func openVPNDomainMatches(suffix string, domain string) bool {
	normalizedSuffix := strings.ToLower(strings.TrimSpace(suffix))
	if normalizedSuffix == "." {
		return true
	}
	normalizedSuffix = strings.TrimSuffix(normalizedSuffix, ".")
	normalizedDomain := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
	if normalizedSuffix == "" {
		return false
	}
	return normalizedDomain == normalizedSuffix || strings.HasSuffix(normalizedDomain, "."+normalizedSuffix)
}
