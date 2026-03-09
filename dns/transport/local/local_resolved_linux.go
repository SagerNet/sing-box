package local

import (
	"bufio"
	"context"
	"errors"
	"net/netip"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	dnsTransport "github.com/sagernet/sing-box/dns/transport"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/service/resolved"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/service"

	"github.com/godbus/dbus/v5"
	mDNS "github.com/miekg/dns"
)

func isSystemdResolvedManaged() bool {
	resolvContent, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return false
	}
	defer resolvContent.Close()
	scanner := bufio.NewScanner(resolvContent)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] != '#' {
			return false
		}
		if strings.Contains(line, "systemd-resolved") {
			return true
		}
	}
	return false
}

type DBusResolvedResolver struct {
	ctx               context.Context
	logger            logger.ContextLogger
	interfaceMonitor  tun.DefaultInterfaceMonitor
	interfaceCallback *list.Element[tun.DefaultInterfaceUpdateCallback]
	systemBus         *dbus.Conn
	savedServerSet    atomic.Pointer[resolvedServerSet]
	closeOnce         sync.Once
}

type resolvedServerSet struct {
	servers []resolvedServer
}

type resolvedServer struct {
	primaryTransport  adapter.DNSTransport
	fallbackTransport adapter.DNSTransport
}

type resolvedServerSpecification struct {
	address    netip.Addr
	port       uint16
	serverName string
}

func NewResolvedResolver(ctx context.Context, logger logger.ContextLogger) (ResolvedResolver, error) {
	interfaceMonitor := service.FromContext[adapter.NetworkManager](ctx).InterfaceMonitor()
	if interfaceMonitor == nil {
		return nil, os.ErrInvalid
	}
	systemBus, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	return &DBusResolvedResolver{
		ctx:              ctx,
		logger:           logger,
		interfaceMonitor: interfaceMonitor,
		systemBus:        systemBus,
	}, nil
}

func (t *DBusResolvedResolver) Start() error {
	t.updateStatus()
	t.interfaceCallback = t.interfaceMonitor.RegisterCallback(t.updateDefaultInterface)
	err := t.systemBus.BusObject().AddMatchSignal(
		"org.freedesktop.DBus",
		"NameOwnerChanged",
		dbus.WithMatchSender("org.freedesktop.DBus"),
		dbus.WithMatchArg(0, "org.freedesktop.resolve1"),
	).Err
	if err != nil {
		return E.Cause(err, "configure resolved restart listener")
	}
	err = t.systemBus.BusObject().AddMatchSignal(
		"org.freedesktop.DBus.Properties",
		"PropertiesChanged",
		dbus.WithMatchSender("org.freedesktop.resolve1"),
		dbus.WithMatchArg(0, "org.freedesktop.resolve1.Manager"),
	).Err
	if err != nil {
		return E.Cause(err, "configure resolved properties listener")
	}
	go t.loopUpdateStatus()
	return nil
}

func (t *DBusResolvedResolver) Close() error {
	var closeErr error
	t.closeOnce.Do(func() {
		serverSet := t.savedServerSet.Swap(nil)
		if serverSet != nil {
			closeErr = serverSet.Close()
		}
		if t.interfaceCallback != nil {
			t.interfaceMonitor.UnregisterCallback(t.interfaceCallback)
		}
		if t.systemBus != nil {
			_ = t.systemBus.Close()
		}
	})
	return closeErr
}

func (t *DBusResolvedResolver) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	serverSet := t.savedServerSet.Load()
	if serverSet == nil {
		var err error
		serverSet, err = t.checkResolved(context.Background())
		if err != nil {
			return nil, err
		}
		previousServerSet := t.savedServerSet.Swap(serverSet)
		if previousServerSet != nil {
			_ = previousServerSet.Close()
		}
	}
	response, err := t.exchangeServerSet(ctx, message, serverSet)
	if err == nil {
		return response, nil
	}
	t.updateStatus()
	refreshedServerSet := t.savedServerSet.Load()
	if refreshedServerSet == nil || refreshedServerSet == serverSet {
		return nil, err
	}
	return t.exchangeServerSet(ctx, message, refreshedServerSet)
}

func (t *DBusResolvedResolver) loopUpdateStatus() {
	signalChan := make(chan *dbus.Signal, 1)
	t.systemBus.Signal(signalChan)
	for signal := range signalChan {
		switch signal.Name {
		case "org.freedesktop.DBus.NameOwnerChanged":
			if len(signal.Body) != 3 {
				continue
			}
			newOwner, loaded := signal.Body[2].(string)
			if !loaded || newOwner == "" {
				continue
			}
			t.updateStatus()
		case "org.freedesktop.DBus.Properties.PropertiesChanged":
			if !shouldUpdateResolvedServerSet(signal) {
				continue
			}
			t.updateStatus()
		}
	}
}

func (t *DBusResolvedResolver) updateStatus() {
	serverSet, err := t.checkResolved(context.Background())
	oldServerSet := t.savedServerSet.Swap(serverSet)
	if oldServerSet != nil {
		_ = oldServerSet.Close()
	}
	if err != nil {
		var dbusErr dbus.Error
		if !errors.As(err, &dbusErr) || dbusErr.Name != "org.freedesktop.DBus.Error.NameHasNoOwner" {
			t.logger.Debug(E.Cause(err, "systemd-resolved service unavailable"))
		}
		if oldServerSet != nil {
			t.logger.Debug("systemd-resolved service is gone")
		}
		return
	} else if oldServerSet == nil {
		t.logger.Debug("using systemd-resolved service as resolver")
	}
}

func (t *DBusResolvedResolver) exchangeServerSet(ctx context.Context, message *mDNS.Msg, serverSet *resolvedServerSet) (*mDNS.Msg, error) {
	if serverSet == nil || len(serverSet.servers) == 0 {
		return nil, E.New("link has no DNS servers configured")
	}
	var lastError error
	for _, server := range serverSet.servers {
		response, err := server.primaryTransport.Exchange(ctx, message)
		if err != nil && server.fallbackTransport != nil {
			response, err = server.fallbackTransport.Exchange(ctx, message)
		}
		if err != nil {
			lastError = err
			continue
		}
		return response, nil
	}
	return nil, lastError
}

func (t *DBusResolvedResolver) checkResolved(ctx context.Context) (*resolvedServerSet, error) {
	dbusObject := t.systemBus.Object("org.freedesktop.resolve1", "/org/freedesktop/resolve1")
	err := dbusObject.Call("org.freedesktop.DBus.Peer.Ping", 0).Err
	if err != nil {
		return nil, err
	}
	defaultInterface := t.interfaceMonitor.DefaultInterface()
	if defaultInterface == nil {
		return nil, E.New("missing default interface")
	}
	call := dbusObject.(*dbus.Object).CallWithContext(
		ctx,
		"org.freedesktop.resolve1.Manager.GetLink",
		0,
		int32(defaultInterface.Index),
	)
	if call.Err != nil {
		return nil, call.Err
	}
	var linkPath dbus.ObjectPath
	err = call.Store(&linkPath)
	if err != nil {
		return nil, err
	}
	linkObject := t.systemBus.Object("org.freedesktop.resolve1", linkPath)
	if linkObject == nil {
		return nil, E.New("missing link object for default interface")
	}
	dnsOverTLSMode, err := loadResolvedLinkDNSOverTLS(linkObject)
	if err != nil {
		return nil, err
	}
	linkDNSEx, err := loadResolvedLinkDNSEx(linkObject)
	if err != nil {
		return nil, err
	}
	linkDNS, err := loadResolvedLinkDNS(linkObject)
	if err != nil {
		return nil, err
	}
	if len(linkDNSEx) == 0 && len(linkDNS) == 0 {
		for _, inbound := range service.FromContext[adapter.InboundManager](t.ctx).Inbounds() {
			if inbound.Type() == C.TypeTun {
				return nil, E.New("No appropriate name servers or networks for name found")
			}
		}
		return nil, E.New("link has no DNS servers configured")
	}
	serverDialer, err := dialer.NewDefault(t.ctx, option.DialerOptions{
		BindInterface:      defaultInterface.Name,
		UDPFragmentDefault: true,
	})
	if err != nil {
		return nil, err
	}
	var serverSpecifications []resolvedServerSpecification
	if len(linkDNSEx) > 0 {
		for _, entry := range linkDNSEx {
			serverSpecification, loaded := buildResolvedServerSpecification(defaultInterface.Name, entry.Address, entry.Port, entry.Name)
			if !loaded {
				continue
			}
			serverSpecifications = append(serverSpecifications, serverSpecification)
		}
	} else {
		for _, entry := range linkDNS {
			serverSpecification, loaded := buildResolvedServerSpecification(defaultInterface.Name, entry.Address, 0, "")
			if !loaded {
				continue
			}
			serverSpecifications = append(serverSpecifications, serverSpecification)
		}
	}
	if len(serverSpecifications) == 0 {
		return nil, E.New("no valid DNS servers on link")
	}
	serverSet := &resolvedServerSet{
		servers: make([]resolvedServer, 0, len(serverSpecifications)),
	}
	for _, serverSpecification := range serverSpecifications {
		server, createErr := t.createResolvedServer(serverDialer, dnsOverTLSMode, serverSpecification)
		if createErr != nil {
			_ = serverSet.Close()
			return nil, createErr
		}
		serverSet.servers = append(serverSet.servers, server)
	}
	return serverSet, nil
}

func (t *DBusResolvedResolver) createResolvedServer(serverDialer N.Dialer, dnsOverTLSMode string, serverSpecification resolvedServerSpecification) (resolvedServer, error) {
	if dnsOverTLSMode == "yes" {
		primaryTransport, err := t.createResolvedTransport(serverDialer, serverSpecification, true)
		if err != nil {
			return resolvedServer{}, err
		}
		return resolvedServer{
			primaryTransport: primaryTransport,
		}, nil
	}
	if dnsOverTLSMode == "opportunistic" {
		primaryTransport, err := t.createResolvedTransport(serverDialer, serverSpecification, true)
		if err != nil {
			return resolvedServer{}, err
		}
		fallbackTransport, err := t.createResolvedTransport(serverDialer, serverSpecification, false)
		if err != nil {
			_ = primaryTransport.Close()
			return resolvedServer{}, err
		}
		return resolvedServer{
			primaryTransport:  primaryTransport,
			fallbackTransport: fallbackTransport,
		}, nil
	}
	primaryTransport, err := t.createResolvedTransport(serverDialer, serverSpecification, false)
	if err != nil {
		return resolvedServer{}, err
	}
	return resolvedServer{
		primaryTransport: primaryTransport,
	}, nil
}

func (t *DBusResolvedResolver) createResolvedTransport(serverDialer N.Dialer, serverSpecification resolvedServerSpecification, useTLS bool) (adapter.DNSTransport, error) {
	serverAddress := M.SocksaddrFrom(serverSpecification.address, resolvedServerPort(serverSpecification.port, useTLS))
	if useTLS {
		tlsAddress := serverSpecification.address
		if tlsAddress.Zone() != "" {
			tlsAddress = tlsAddress.WithZone("")
		}
		serverName := serverSpecification.serverName
		if serverName == "" {
			serverName = tlsAddress.String()
		}
		tlsConfig, err := tls.NewClient(t.ctx, t.logger, tlsAddress.String(), option.OutboundTLSOptions{
			Enabled:    true,
			ServerName: serverName,
		})
		if err != nil {
			return nil, err
		}
		serverTransport := dnsTransport.NewTLSRaw(t.logger, dns.NewTransportAdapter(C.DNSTypeTLS, "", nil), serverDialer, serverAddress, tlsConfig)
		err = serverTransport.Start(adapter.StartStateStart)
		if err != nil {
			_ = serverTransport.Close()
			return nil, err
		}
		return serverTransport, nil
	}
	serverTransport := dnsTransport.NewUDPRaw(t.logger, dns.NewTransportAdapter(C.DNSTypeUDP, "", nil), serverDialer, serverAddress)
	err := serverTransport.Start(adapter.StartStateStart)
	if err != nil {
		_ = serverTransport.Close()
		return nil, err
	}
	return serverTransport, nil
}

func (s *resolvedServerSet) Close() error {
	var errors []error
	for _, server := range s.servers {
		errors = append(errors, server.primaryTransport.Close())
		if server.fallbackTransport != nil {
			errors = append(errors, server.fallbackTransport.Close())
		}
	}
	return E.Errors(errors...)
}

func buildResolvedServerSpecification(interfaceName string, rawAddress []byte, port uint16, serverName string) (resolvedServerSpecification, bool) {
	address, loaded := netip.AddrFromSlice(rawAddress)
	if !loaded {
		return resolvedServerSpecification{}, false
	}
	if address.Is6() && address.IsLinkLocalUnicast() && address.Zone() == "" {
		address = address.WithZone(interfaceName)
	}
	return resolvedServerSpecification{
		address:    address,
		port:       port,
		serverName: serverName,
	}, true
}

func resolvedServerPort(port uint16, useTLS bool) uint16 {
	if port > 0 {
		return port
	}
	if useTLS {
		return 853
	}
	return 53
}

func loadResolvedLinkDNS(linkObject dbus.BusObject) ([]resolved.LinkDNS, error) {
	dnsProperty, err := linkObject.GetProperty("org.freedesktop.resolve1.Link.DNS")
	if err != nil {
		if isResolvedUnknownPropertyError(err) {
			return nil, nil
		}
		return nil, err
	}
	var linkDNS []resolved.LinkDNS
	err = dnsProperty.Store(&linkDNS)
	if err != nil {
		return nil, err
	}
	return linkDNS, nil
}

func loadResolvedLinkDNSEx(linkObject dbus.BusObject) ([]resolved.LinkDNSEx, error) {
	dnsProperty, err := linkObject.GetProperty("org.freedesktop.resolve1.Link.DNSEx")
	if err != nil {
		if isResolvedUnknownPropertyError(err) {
			return nil, nil
		}
		return nil, err
	}
	var linkDNSEx []resolved.LinkDNSEx
	err = dnsProperty.Store(&linkDNSEx)
	if err != nil {
		return nil, err
	}
	return linkDNSEx, nil
}

func loadResolvedLinkDNSOverTLS(linkObject dbus.BusObject) (string, error) {
	dnsOverTLSProperty, err := linkObject.GetProperty("org.freedesktop.resolve1.Link.DNSOverTLS")
	if err != nil {
		if isResolvedUnknownPropertyError(err) {
			return "", nil
		}
		return "", err
	}
	var dnsOverTLSMode string
	err = dnsOverTLSProperty.Store(&dnsOverTLSMode)
	if err != nil {
		return "", err
	}
	return dnsOverTLSMode, nil
}

func isResolvedUnknownPropertyError(err error) bool {
	var dbusError dbus.Error
	return errors.As(err, &dbusError) && dbusError.Name == "org.freedesktop.DBus.Error.UnknownProperty"
}

func shouldUpdateResolvedServerSet(signal *dbus.Signal) bool {
	if len(signal.Body) != 3 {
		return true
	}
	changedProperties, loaded := signal.Body[1].(map[string]dbus.Variant)
	if !loaded {
		return true
	}
	for propertyName := range changedProperties {
		switch propertyName {
		case "DNS", "DNSEx", "DNSOverTLS":
			return true
		}
	}
	invalidatedProperties, loaded := signal.Body[2].([]string)
	if !loaded {
		return true
	}
	for _, propertyName := range invalidatedProperties {
		switch propertyName {
		case "DNS", "DNSEx", "DNSOverTLS":
			return true
		}
	}
	return false
}

func (t *DBusResolvedResolver) updateDefaultInterface(defaultInterface *control.Interface, flags int) {
	t.updateStatus()
}
