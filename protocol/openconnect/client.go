package openconnect

import (
	"context"
	"crypto/tls"
	"net"
	"net/netip"
	"net/url"
	"strconv"
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
	openconnecttransport "github.com/sagernet/sing-box/transport/openconnect"
	"github.com/sagernet/sing-openconnect"
	"github.com/sagernet/sing-tun"
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
	_ adapter.OutboundWithPreferredRoutes = (*Endpoint)(nil)
	_ adapter.FlowOutbound                = (*Endpoint)(nil)
	_ adapter.InterfaceUpdateListener     = (*Endpoint)(nil)
	_ dialer.PacketDialerWithDestination  = (*Endpoint)(nil)
	_ tun.Port                            = (*Endpoint)(nil)
)

type Endpoint struct {
	endpointBase
	loopContext             context.Context
	cancelLoop              context.CancelFunc
	dnsRouter               adapter.DNSRouter
	client                  *openconnect.Client
	device                  openconnecttransport.Device
	server                  string
	flavor                  string
	stateAccess             sync.Mutex
	state                   atomic.Pointer[clientState]
	dnsTransportAccess      sync.Mutex
	dnsTransport            *DNSTransport
	deviceStarted           bool
	readLoopDone            chan struct{}
	statusAccess            sync.Mutex
	statusUpdated           chan struct{}
	terminalError           string
	authFormLoopDone        chan struct{}
	activeTransportLoopDone chan struct{}
	hotpCounter             atomic.Uint64
}

type clientState struct {
	started          bool
	tunnelConfigured bool
	localAddresses   []netip.Prefix
	routeSet         *netipx.IPSet
	preferredDomains map[string]bool
	configuration    openconnecttransport.Configuration
	tunnelInfo       adapter.OpenConnectTunnelInfo
}

func NewEndpoint(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.OpenConnectEndpointOptions) (adapter.Endpoint, error) {
	tcpKeepAliveEnabled := options.TCPKeepAliveEnabled || options.TCPKeepAlive != 0 || options.TCPKeepAliveInterval != 0
	if tcpKeepAliveEnabled && options.DisableTCPKeepAlive {
		return nil, E.New("tcp_keep_alive_enabled conflicts with disable_tcp_keep_alive")
	}
	if !tcpKeepAliveEnabled {
		options.DisableTCPKeepAlive = true
	} else if options.TCPKeepAlive == 0 && options.TCPKeepAliveInterval == 0 {
		options.TCPKeepAliveSystemDefaults = true
	}
	options.UDPBindPort = options.DTLSLocalPort
	loopContext, cancelLoop := context.WithCancel(ctx)
	openConnectEndpoint := &Endpoint{
		endpointBase: endpointBase{
			Adapter: endpoint.NewAdapterWithDialerOptions(C.TypeOpenConnect, tag, []string{N.NetworkTCP, N.NetworkUDP, N.NetworkICMP}, options.DialerOptions),
			router:  router,
			logger:  logger,
		},
		loopContext:   loopContext,
		cancelLoop:    cancelLoop,
		dnsRouter:     service.FromContext[adapter.DNSRouter](ctx),
		statusUpdated: make(chan struct{}),
	}
	openConnectEndpoint.state.Store(new(clientState))
	success := false
	defer func() {
		if success {
			return
		}
		if openConnectEndpoint.device != nil {
			_ = openConnectEndpoint.device.Close()
		}
		cancelLoop()
	}()
	server := options.Server
	if !strings.Contains(server, "://") {
		server = "https://" + server
	}
	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, E.Cause(err, "parse server")
	}
	serverPort := serverURL.Port()
	if serverPort == "" {
		serverPort = "443"
	}
	openConnectEndpoint.server = net.JoinHostPort(serverURL.Hostname(), serverPort)
	openConnectEndpoint.flavor = options.Flavor
	if openConnectEndpoint.flavor == "" {
		openConnectEndpoint.flavor = openconnect.FlavorAnyConnect
	}
	serverAddress, serverAddressErr := netip.ParseAddr(serverURL.Hostname())
	remoteIsDomain := serverURL.Hostname() != "" && serverAddressErr != nil && !serverAddress.IsValid()
	outboundDialer, err := dialer.NewWithOptions(dialer.Options{
		Context:          ctx,
		Options:          options.DialerOptions,
		RemoteIsDomain:   remoteIsDomain,
		ResolverOnDetour: true,
		NewDialer:        true,
	})
	if err != nil {
		return nil, err
	}
	udpTimeout := C.UDPTimeout
	if options.UDPTimeout != 0 {
		udpTimeout = time.Duration(options.UDPTimeout)
	}
	networkManager := service.FromContext[adapter.NetworkManager](ctx)
	device, err := openconnecttransport.NewDevice(openconnecttransport.DeviceOptions{
		Context:         ctx,
		Logger:          logger,
		System:          options.System,
		Handler:         openConnectEndpoint,
		UDPTimeout:      udpTimeout,
		ICMPTimeout:     C.ICMPTimeout,
		UDPMapping:      tun.NATMapping(options.UDPMapping),
		UDPFiltering:    tun.NATFiltering(options.UDPFiltering),
		UDPNATMax:       options.UDPNATMax,
		InterfaceFinder: networkManager.InterfaceFinder(),
		Name:            options.Name,
		MTU:             openconnecttransport.DefaultMTU,
		Configuration: openconnecttransport.Configuration{
			MTU: openconnecttransport.DefaultMTU,
		},
	})
	if err != nil {
		return nil, err
	}
	openConnectEndpoint.device = device
	device.SetPacketWriter(openConnectEndpoint.writePacketBuffers)
	clientOptions, err := openConnectEndpoint.buildClientOptions(options, outboundDialer)
	if err != nil {
		return nil, err
	}
	client, err := openconnect.NewClient(clientOptions)
	if err != nil {
		return nil, err
	}
	openConnectEndpoint.client = client
	success = true
	return openConnectEndpoint, nil
}

func (e *Endpoint) buildClientOptions(options option.OpenConnectEndpointOptions, outboundDialer N.Dialer) (openconnect.ClientOptions, error) {
	var tlsConfig *tls.Config
	if options.TLS.Insecure {
		tlsConfig = &tls.Config{InsecureSkipVerify: true}
	}
	certificateAuthority, err := materialSource("tls.certificate_authority", options.TLS.CertificateAuthority, options.TLS.CertificateAuthorityPath)
	if err != nil {
		return openconnect.ClientOptions{}, err
	}
	clientCertificate, err := materialSource("tls.client_certificate", options.TLS.ClientCertificate, options.TLS.ClientCertificatePath)
	if err != nil {
		return openconnect.ClientOptions{}, err
	}
	clientKey, err := materialSource("tls.client_key", options.TLS.ClientKey, options.TLS.ClientKeyPath)
	if err != nil {
		return openconnect.ClientOptions{}, err
	}
	mcaCertificate, err := materialSource("tls.mca_certificate", options.TLS.MCACertificate, options.TLS.MCACertificatePath)
	if err != nil {
		return openconnect.ClientOptions{}, err
	}
	mcaKey, err := materialSource("tls.mca_key", options.TLS.MCAKey, options.TLS.MCAKeyPath)
	if err != nil {
		return openconnect.ClientOptions{}, err
	}
	var tokenOptions *openconnect.TokenOptions
	if options.Token != nil {
		tokenOptions = &openconnect.TokenOptions{
			Mode:       options.Token.Mode,
			Secret:     options.Token.Secret,
			SecretPath: options.Token.SecretPath,
			PIN:        options.Token.PIN,
			Password:   options.Token.Password,
			DeviceID:   options.Token.DeviceID,
			Counter:    options.Token.Counter,
		}
		if tokenOptions.Mode == openconnect.TokenModeHOTP {
			e.hotpCounter.Store(tokenOptions.Counter)
			tokenOptions.UpdateCounter = func(_ context.Context, counter uint64) error {
				e.hotpCounter.Store(counter)
				return nil
			}
		}
	}
	var csdOptions *openconnect.CSDOptions
	var mobileOptions *openconnect.MobileOptions
	if options.Mobile != nil {
		mobileOptions = &openconnect.MobileOptions{
			PlatformVersion: options.Mobile.PlatformVersion,
			DeviceType:      options.Mobile.DeviceType,
			DeviceUniqueID:  options.Mobile.DeviceUniqueID,
		}
	}
	if options.CSD != nil {
		csdOptions = &openconnect.CSDOptions{WrapperPath: options.CSD.WrapperPath}
	}
	var hipOptions *openconnect.HIPOptions
	if options.HIP != nil {
		hipOptions = &openconnect.HIPOptions{WrapperPath: options.HIP.WrapperPath}
	}
	var tnccOptions *openconnect.TNCCOptions
	if options.TNCC != nil {
		tnccCertificates := make([]openconnect.Material, 0, len(options.TNCC.Certificates))
		for i, certificateOptions := range options.TNCC.Certificates {
			certificate, certificateErr := materialSource("tncc.certificates["+strconv.Itoa(i)+"].certificate", certificateOptions.Certificate, certificateOptions.CertificatePath)
			if certificateErr != nil {
				return openconnect.ClientOptions{}, certificateErr
			}
			tnccCertificates = append(tnccCertificates, certificate)
		}
		tnccOptions = &openconnect.TNCCOptions{
			WrapperPath:                  options.TNCC.WrapperPath,
			DeviceID:                     options.TNCC.DeviceID,
			UserAgent:                    options.TNCC.UserAgent,
			MachineIdentificationEnabled: options.TNCC.MachineIdentificationEnabled,
			Certificates:                 tnccCertificates,
		}
	}
	formEntries := common.Map(options.FormEntries, func(entry option.OpenConnectFormEntryOptions) openconnect.FormEntry {
		return openconnect.FormEntry{
			FormID:        entry.FormID,
			SubmissionKey: entry.SubmissionKey,
			Name:          entry.Name,
			Value:         entry.Value,
			Promote:       entry.Promote,
		}
	})
	return openconnect.ClientOptions{
		Context:                        e.loopContext,
		Server:                         options.Server,
		Flavor:                         options.Flavor,
		Username:                       options.Username,
		Password:                       options.Password,
		AuthGroup:                      options.AuthGroup,
		Cookie:                         options.Cookie,
		Token:                          tokenOptions,
		ReportedOS:                     options.ReportedOS,
		UserAgent:                      options.UserAgent,
		Version:                        options.Version,
		LocalHostname:                  options.LocalHostname,
		Mobile:                         mobileOptions,
		CSD:                            csdOptions,
		HIP:                            hipOptions,
		TNCC:                           tnccOptions,
		NoUDP:                          options.NoUDP,
		DTLSLocalPort:                  options.DTLSLocalPort,
		CompressionDisabled:            options.CompressionDisabled,
		CompressionMode:                options.CompressionMode,
		IPv6Disabled:                   options.IPv6Disabled,
		HTTPKeepAliveDisabled:          options.HTTPKeepAliveDisabled,
		XMLPostDisabled:                options.XMLPostDisabled,
		ExternalAuthDisabled:           options.ExternalAuthDisabled,
		PasswordAuthenticationDisabled: options.PasswordAuthenticationDisabled,
		PFS:                            options.PFS,
		MTU:                            options.MTU,
		BaseMTU:                        options.BaseMTU,
		DPDInterval:                    time.Duration(options.DPDInterval),
		ReconnectTimeout:               time.Duration(options.ReconnectTimeout),
		TrojanInterval:                 time.Duration(options.TrojanInterval),
		QueueLength:                    options.QueueLength,
		AllowInsecureCrypto:            options.AllowInsecureCrypto,
		TLSConfig: openconnect.ClientTLSOptions{
			Config:               tlsConfig,
			ServerName:           options.TLS.ServerName,
			PeerFingerprints:     options.TLS.PeerFingerprint,
			SystemTrustDisabled:  options.TLS.SystemTrustDisabled,
			CertificateAuthority: certificateAuthority,
			Certificate:          clientCertificate,
			Key:                  clientKey,
			KeyPassword:          options.TLS.ClientKeyPassword,
			MCACertificate:       mcaCertificate,
			MCAKey:               mcaKey,
			MCAKeyPassword:       options.TLS.MCAKeyPassword,
		},
		FormEntries:           formEntries,
		Dialer:                outboundDialer,
		Logger:                e.logger,
		OnTunnelConfiguration: e.handleTunnelConfiguration,
	}, nil
}

func (e *Endpoint) handleTunnelConfiguration(event openconnect.TunnelConfigurationEvent) error {
	configuration := configurationFromClientEvent(event)
	defer e.notifyStatusUpdated()
	e.stateAccess.Lock()
	defer e.stateAccess.Unlock()
	e.updateState(func(state *clientState) {
		state.tunnelConfigured = false
	})
	routeSet, err := buildIPSet(configuration.Routes, configuration.ExcludedRoutes)
	if err != nil {
		return E.Cause(err, "build route set")
	}
	err = e.device.UpdateConfiguration(openconnecttransport.Configuration{
		MTU:       configuration.MTU,
		Addresses: configuration.Addresses,
	})
	if err != nil {
		return E.Cause(err, "update device configuration")
	}
	if !e.deviceStarted {
		err = e.device.Start()
		if err != nil {
			return E.Cause(err, "start device")
		}
		e.deviceStarted = true
	}
	preferredDomains := buildPreferredDomains(configuration)
	var ipv4Addresses []netip.Prefix
	var ipv6Addresses []netip.Prefix
	for _, address := range configuration.Addresses {
		if address.Addr().Is4() {
			ipv4Addresses = append(ipv4Addresses, address)
		} else if address.Addr().Is6() {
			ipv6Addresses = append(ipv6Addresses, address)
		}
	}
	e.updateState(func(state *clientState) {
		connectedSince := state.tunnelInfo.ConnectedSince
		if event.Reason == openconnect.TunnelConfigurationEventInitial ||
			event.Reason == openconnect.TunnelConfigurationEventReestablishment ||
			connectedSince.IsZero() {
			connectedSince = time.Now()
		}
		state.tunnelConfigured = true
		state.localAddresses = configuration.Addresses
		state.routeSet = routeSet
		state.preferredDomains = preferredDomains
		state.configuration = configuration
		state.tunnelInfo = adapter.OpenConnectTunnelInfo{
			Server:         e.server,
			Flavor:         e.flavor,
			Transport:      state.tunnelInfo.Transport,
			IPv4:           ipv4Addresses,
			IPv6:           ipv6Addresses,
			DNS:            configuration.DNS,
			MTU:            configuration.MTU,
			ConnectedSince: connectedSince,
		}
	})
	e.dnsTransportAccess.Lock()
	dnsTransport := e.dnsTransport
	e.dnsTransportAccess.Unlock()
	if dnsTransport != nil {
		dnsTransport.updateConfiguration(configuration)
	}
	return nil
}

func (e *Endpoint) updateState(update func(state *clientState)) {
	newState := *e.state.Load()
	update(&newState)
	e.state.Store(&newState)
}

func (e *Endpoint) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStatePostStart {
		return nil
	}
	err := e.client.Start()
	if err != nil {
		return err
	}
	e.stateAccess.Lock()
	e.updateState(func(state *clientState) {
		state.started = true
	})
	e.readLoopDone = make(chan struct{})
	e.authFormLoopDone = make(chan struct{})
	e.activeTransportLoopDone = make(chan struct{})
	e.stateAccess.Unlock()
	go e.readLoop()
	go e.watchAuthForms()
	go e.watchActiveTransport()
	return nil
}

func (e *Endpoint) readLoop() {
	defer close(e.readLoopDone)
	for {
		packetBuffers, err := e.client.ReadDataPackets(e.loopContext)
		if err != nil {
			if E.IsClosedOrCanceled(err) || e.loopContext.Err() != nil {
				return
			}
			e.logger.Error(E.Cause(err, "client terminated"))
			e.setTerminalError(err)
			return
		}
		err = e.device.WriteInboundBuffers(packetBuffers)
		buf.ReleaseMulti(packetBuffers)
		if err != nil {
			err = E.Cause(err, "write packet to device")
			e.logger.Error(err)
			e.setTerminalError(err)
			return
		}
	}
}

func (e *Endpoint) Close() error {
	e.stateAccess.Lock()
	e.updateState(func(state *clientState) {
		state.started = false
	})
	readLoopDone := e.readLoopDone
	authFormLoopDone := e.authFormLoopDone
	activeTransportLoopDone := e.activeTransportLoopDone
	e.stateAccess.Unlock()
	e.cancelLoop()
	err := E.Errors(e.client.Close(), e.device.Close())
	if readLoopDone != nil {
		<-readLoopDone
	}
	if authFormLoopDone != nil {
		<-authFormLoopDone
	}
	if activeTransportLoopDone != nil {
		<-activeTransportLoopDone
	}
	e.notifyStatusUpdated()
	return err
}

func (e *Endpoint) InterfaceUpdated() {
	e.client.RestartSession()
}

func (e *Endpoint) PreMatchFlow(network string, destination netip.Addr) adapter.PreMatchAction {
	return adapter.PreMatchFlow
}

func (e *Endpoint) PortAddresses() (netip.Addr, netip.Addr) {
	return e.device.PortAddresses()
}

func (e *Endpoint) PortMTU() uint32 {
	return e.device.PortMTU()
}

func (e *Endpoint) AttachReturn(returnPath tun.Return) error {
	return e.device.AttachReturn(returnPath)
}

func (e *Endpoint) DetachReturn(returnPath tun.Return) error {
	return e.device.DetachReturn(returnPath)
}

func (e *Endpoint) JudgeFlow(network uint8, source netip.AddrPort, destination netip.AddrPort, firstPacket []byte) tun.FlowVerdict {
	return judgeOpenConnectFlow(e.router, e.Tag(), e.Type(), e.state.Load().localAddresses, network, source, destination, firstPacket)
}

func (e *Endpoint) NewDNSPacket(payload []byte, source M.Socksaddr, destination M.Socksaddr, writer N.PacketWriter) {
	e.newDNSPacket(log.ContextWithNewID(e.loopContext), e, payload, source, destination, writer)
}

func (e *Endpoint) ready() bool {
	state := e.state.Load()
	return state.started && state.tunnelConfigured
}

func (e *Endpoint) WritePackets(packets [][]byte) error {
	if !e.ready() {
		return E.New("endpoint is not ready yet")
	}
	err := e.client.WriteDataPackets(packets)
	if E.IsMulti(err, openconnect.ErrDataChannelNotReady) {
		return E.New("endpoint is not ready yet")
	}
	return err
}

func (e *Endpoint) writePacketBuffers(packetBuffers []*buf.Buffer) error {
	if !e.ready() {
		buf.ReleaseMulti(packetBuffers)
		return nil
	}
	err := e.client.WriteDataPacketBuffers(packetBuffers)
	if E.IsMulti(err, openconnect.ErrDataChannelNotReady) {
		return nil
	}
	return err
}

func (e *Endpoint) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	e.newConnection(ctx, e, e.state.Load().localAddresses, conn, source, destination, onClose)
}

func (e *Endpoint) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	e.newPacketConnection(ctx, e, e.state.Load().localAddresses, conn, source, destination, onClose)
}

func (e *Endpoint) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	switch network {
	case N.NetworkTCP:
		e.logger.InfoContext(ctx, "outbound connection to ", destination)
	case N.NetworkUDP:
		e.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	}
	if !e.ready() || !e.client.Ready() {
		return nil, E.New("endpoint is not ready yet")
	}
	if destination.IsDomain() {
		destinationAddresses, err := e.dnsRouter.Lookup(ctx, destination.Fqdn, adapter.DNSQueryOptions{})
		if err != nil {
			return nil, err
		}
		return N.DialSerial(ctx, e.device, network, destination, destinationAddresses)
	}
	if !destination.Addr.IsValid() {
		return nil, E.New("invalid destination: ", destination)
	}
	return e.device.DialContext(ctx, network, destination)
}

func (e *Endpoint) ListenPacketWithDestination(ctx context.Context, destination M.Socksaddr) (net.PacketConn, netip.Addr, error) {
	e.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	if !e.ready() || !e.client.Ready() {
		return nil, netip.Addr{}, E.New("endpoint is not ready yet")
	}
	if destination.IsDomain() {
		destinationAddresses, err := e.dnsRouter.Lookup(ctx, destination.Fqdn, adapter.DNSQueryOptions{})
		if err != nil {
			return nil, netip.Addr{}, err
		}
		return N.ListenSerial(ctx, e.device, destination, destinationAddresses)
	}
	packetConn, err := e.device.ListenPacket(ctx, destination)
	if err != nil {
		return nil, netip.Addr{}, err
	}
	if destination.IsIP() {
		return packetConn, destination.Addr, nil
	}
	return packetConn, netip.Addr{}, nil
}

func (e *Endpoint) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	packetConn, destinationAddress, err := e.ListenPacketWithDestination(ctx, destination)
	if err != nil {
		return nil, err
	}
	if destinationAddress.IsValid() && destination != M.SocksaddrFrom(destinationAddress, destination.Port) {
		return bufio.NewNATPacketConn(bufio.NewPacketConn(packetConn), M.SocksaddrFrom(destinationAddress, destination.Port), destination), nil
	}
	return packetConn, nil
}

func (e *Endpoint) PreferredDomain(metadata *adapter.InboundContext, domain string) bool {
	state := e.state.Load()
	if !state.started || !state.tunnelConfigured || !e.client.Ready() {
		return false
	}
	canonicalDomain := canonicalOpenConnectDomain(domain)
	return openConnectDomainMatchesAny(canonicalDomain, state.preferredDomains)
}

func (e *Endpoint) PreferredAddress(metadata *adapter.InboundContext, address netip.Addr) bool {
	state := e.state.Load()
	if !state.started || !state.tunnelConfigured || state.routeSet == nil || !e.client.Ready() {
		return false
	}
	return state.routeSet.Contains(address)
}
