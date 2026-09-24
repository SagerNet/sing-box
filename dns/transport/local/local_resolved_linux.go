package local

import (
	"bufio"
	"context"
	"errors"
	"net/netip"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	dnsTransport "github.com/sagernet/sing-box/dns/transport"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
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
	interfaceFinder   control.InterfaceFinder
	interfaceMonitor  tun.DefaultInterfaceMonitor
	interfaceCallback *list.Element[tun.DefaultInterfaceUpdateCallback]
	networkMonitor    tun.NetworkUpdateMonitor
	networkCallback   *list.Element[tun.NetworkUpdateCallback]
	systemBus         *dbus.Conn
	savedServerSet    atomic.Pointer[resolvedServerSet]
	updateAccess      sync.Mutex
	updateCancel      context.CancelFunc
	updateRunAccess   sync.Mutex
	closed            bool
	closeOnce         sync.Once
}

type resolvedServerSet struct {
	scopes          []resolvedScope
	serverAddresses []netip.Addr
	signature       []string
}

// Match levels of dns_scope_good_domain() in systemd-resolved: a routing or search
// domain match yields its label count, and only scopes at the best level are queried.
const (
	resolvedScopeNoMatch    = -3
	resolvedScopeLastResort = -2
	resolvedScopeMaybe      = -1
)

// systemd-resolved only signals changes of the DNS server list, while clients such as
// NetworkManager and systemd-networkd configure domains, default route and DNS over TLS
// of a link in separate calls right after it; link state changes are not signaled at all.
const resolvedUpdateDelay = time.Second

// SD_RESOLVED_DNS bit of org.freedesktop.resolve1.Link.ScopesMask, set when the link has a unicast DNS scope.
const resolvedScopesMaskDNS = 1 << 0

type resolvedScope struct {
	domains      []string
	defaultRoute bool
	fallback     bool
	servers      []resolvedServer
}

type resolvedServer struct {
	primaryTransport  adapter.DNSTransport
	fallbackTransport adapter.DNSTransport
}

type resolvedScopeSpecification struct {
	interfaceName  string
	dnsOverTLSMode string
	domains        []string
	defaultRoute   bool
	fallback       bool
	servers        []resolvedServerSpecification
}

type resolvedServerSpecification struct {
	address    netip.Addr
	port       uint16
	serverName string
}

type resolvedManagerDNS struct {
	InterfaceIndex int32
	Family         int32
	Address        []byte
}

type resolvedManagerDNSEx struct {
	InterfaceIndex int32
	Family         int32
	Address        []byte
	Port           uint16
	Name           string
}

type resolvedManagerDomain struct {
	InterfaceIndex int32
	Domain         string
	RoutingOnly    bool
}

type resolvedManagerServer struct {
	interfaceIndex int32
	address        []byte
	port           uint16
	serverName     string
}

func NewResolvedResolver(ctx context.Context, logger logger.ContextLogger) (ResolvedResolver, error) {
	networkManager := service.FromContext[adapter.NetworkManager](ctx)
	interfaceMonitor := networkManager.InterfaceMonitor()
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
		interfaceFinder:  networkManager.InterfaceFinder(),
		interfaceMonitor: interfaceMonitor,
		networkMonitor:   networkManager.NetworkMonitor(),
		systemBus:        systemBus,
	}, nil
}

func (t *DBusResolvedResolver) Start() error {
	t.updateStatus(t.ctx)
	t.interfaceCallback = t.interfaceMonitor.RegisterCallback(t.updateDefaultInterface)
	if t.networkMonitor != nil {
		t.networkCallback = t.networkMonitor.RegisterCallback(t.postUpdateStatus)
	}
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
		t.updateAccess.Lock()
		updateCancel := t.updateCancel
		t.updateCancel = nil
		t.updateAccess.Unlock()
		if updateCancel != nil {
			updateCancel()
		}
		t.updateRunAccess.Lock()
		t.closed = true
		serverSet := t.savedServerSet.Swap(nil)
		t.updateRunAccess.Unlock()
		if serverSet != nil {
			closeErr = serverSet.Close()
		}
		if t.interfaceCallback != nil {
			t.interfaceMonitor.UnregisterCallback(t.interfaceCallback)
		}
		if t.networkCallback != nil {
			t.networkMonitor.UnregisterCallback(t.networkCallback)
		}
		if t.systemBus != nil {
			_ = t.systemBus.Close()
		}
	})
	return closeErr
}

func (t *DBusResolvedResolver) Reset() {
	serverSet := t.savedServerSet.Load()
	if serverSet == nil {
		return
	}
	for _, scope := range serverSet.scopes {
		for _, server := range scope.servers {
			server.primaryTransport.Reset()
			if server.fallbackTransport != nil {
				server.fallbackTransport.Reset()
			}
		}
	}
}

func (t *DBusResolvedResolver) Environment() []string {
	serverSet := t.savedServerSet.Load()
	if serverSet == nil {
		return nil
	}
	return serverSet.signature
}

func (t *DBusResolvedResolver) ServerAddresses() []netip.Addr {
	serverSet := t.savedServerSet.Load()
	if serverSet == nil {
		return nil
	}
	return serverSet.serverAddresses
}

func (t *DBusResolvedResolver) ExchangeAsync(ctx context.Context, message *mDNS.Msg, callback func(response *mDNS.Msg, err error)) {
	serverSet := t.savedServerSet.Load()
	if serverSet == nil {
		go func() {
			err := t.updateStatus(t.ctx)
			if err != nil {
				callback(nil, err)
				return
			}
			t.exchangeServerSet(ctx, message, t.savedServerSet.Load(), callback)
		}()
		return
	}
	t.exchangeServerSet(ctx, message, serverSet, func(response *mDNS.Msg, err error) {
		if err == nil {
			callback(response, nil)
			return
		}
		go func() {
			t.updateStatus(t.ctx)
			refreshedServerSet := t.savedServerSet.Load()
			if refreshedServerSet == nil || refreshedServerSet == serverSet {
				callback(nil, err)
				return
			}
			t.exchangeServerSet(ctx, message, refreshedServerSet, callback)
		}()
	})
}

func (t *DBusResolvedResolver) exchangeServerSet(ctx context.Context, message *mDNS.Msg, serverSet *resolvedServerSet, callback func(response *mDNS.Msg, err error)) {
	if serverSet == nil {
		callback(nil, os.ErrClosed)
		return
	}
	servers := serverSet.selectServers(message.Question[0].Name)
	if len(servers) == 0 {
		callback(nil, E.New("no appropriate name servers or networks for name found"))
		return
	}
	serverExchangers := make([]dnsTransport.AsyncExchanger, 0, len(servers))
	for _, server := range servers {
		serverExchangers = append(serverExchangers, func(exchangeCtx context.Context, exchangeCallback func(response *mDNS.Msg, err error)) {
			server.primaryTransport.ExchangeAsync(exchangeCtx, message, func(response *mDNS.Msg, exchangeErr error) {
				if exchangeErr != nil && server.fallbackTransport != nil {
					server.fallbackTransport.ExchangeAsync(exchangeCtx, message, exchangeCallback)
					return
				}
				exchangeCallback(response, exchangeErr)
			})
		})
	}
	dnsTransport.ExchangeSequential(ctx, serverExchangers, nil, callback)
}

func (s *resolvedServerSet) selectServers(name string) []resolvedServer {
	var (
		bestMatch      = resolvedScopeNoMatch
		selectedScopes []resolvedScope
	)
	for _, scope := range s.scopes {
		match := scope.match(name)
		if match == resolvedScopeNoMatch || match < bestMatch {
			continue
		}
		if match > bestMatch {
			bestMatch = match
			selectedScopes = selectedScopes[:0]
		}
		selectedScopes = append(selectedScopes, scope)
	}
	return common.FlatMap(selectedScopes, func(it resolvedScope) []resolvedServer {
		return it.servers
	})
}

func (s *resolvedScope) match(name string) int {
	bestLabels := -1
	for _, domain := range s.domains {
		if mDNS.IsSubDomain(domain, name) {
			bestLabels = max(bestLabels, mDNS.CountLabel(domain))
		}
	}
	if bestLabels >= 0 {
		return bestLabels
	}
	if !s.defaultRoute {
		return resolvedScopeNoMatch
	}
	if s.fallback {
		return resolvedScopeLastResort
	}
	return resolvedScopeMaybe
}

func (s *resolvedServerSet) Close() error {
	return E.Errors(common.Map(s.scopes, resolvedScope.Close)...)
}

func (s resolvedScope) Close() error {
	var errors []error
	for _, server := range s.servers {
		errors = append(errors, server.primaryTransport.Close())
		if server.fallbackTransport != nil {
			errors = append(errors, server.fallbackTransport.Close())
		}
	}
	return E.Errors(errors...)
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
			t.postUpdateStatus()
		case "org.freedesktop.DBus.Properties.PropertiesChanged":
			if !shouldUpdateResolvedServerSet(signal) {
				continue
			}
			t.postUpdateStatus()
		}
	}
}

func (t *DBusResolvedResolver) postUpdateStatus() {
	updateContext, updateCancel := context.WithCancel(t.ctx)
	t.updateAccess.Lock()
	previousCancel := t.updateCancel
	t.updateCancel = updateCancel
	t.updateAccess.Unlock()
	if previousCancel != nil {
		previousCancel()
	}
	go func() {
		defer updateCancel()
		timer := time.NewTimer(resolvedUpdateDelay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-updateContext.Done():
			return
		}
		t.updateStatus(updateContext)
	}()
}

func (t *DBusResolvedResolver) updateStatus(ctx context.Context) error {
	t.updateRunAccess.Lock()
	defer t.updateRunAccess.Unlock()
	if t.closed {
		return os.ErrClosed
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	serverSet, err := t.checkResolved(ctx)
	if t.closed || ctx.Err() != nil {
		if serverSet != nil {
			_ = serverSet.Close()
		}
		if t.closed {
			return os.ErrClosed
		}
		return ctx.Err()
	}
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
		return err
	} else if oldServerSet == nil {
		t.logger.Debug("using systemd-resolved service as resolver")
	}
	return nil
}

func (t *DBusResolvedResolver) checkResolved(ctx context.Context) (*resolvedServerSet, error) {
	managerObject := t.systemBus.Object("org.freedesktop.resolve1", "/org/freedesktop/resolve1")
	err := managerObject.CallWithContext(ctx, "org.freedesktop.DBus.Peer.Ping", 0).Err
	if err != nil {
		return nil, err
	}
	defaultInterface := t.interfaceMonitor.DefaultInterface()
	if defaultInterface == nil {
		return nil, E.New("missing default interface")
	}
	managerProperties, err := loadResolvedProperties(ctx, managerObject, "org.freedesktop.resolve1.Manager")
	if err != nil {
		return nil, err
	}
	servers, err := loadResolvedManagerServers(managerProperties, "DNS")
	if err != nil {
		return nil, err
	}
	var managerDomains []resolvedManagerDomain
	err = storeResolvedProperty(managerProperties, "Domains", &managerDomains)
	if err != nil {
		return nil, err
	}
	var dnsOverTLSMode string
	err = storeResolvedProperty(managerProperties, "DNSOverTLS", &dnsOverTLSMode)
	if err != nil {
		return nil, err
	}
	globalScope := resolvedScopeSpecification{
		interfaceName:  defaultInterface.Name,
		dnsOverTLSMode: dnsOverTLSMode,
		defaultRoute:   true,
	}
	var linkScopes []resolvedScopeSpecification
	linkScopeIndexes := make(map[int32]int)
	for _, server := range servers {
		if server.interfaceIndex == 0 {
			serverSpecification, loaded := buildResolvedServerSpecification(globalScope.interfaceName, server)
			if loaded {
				globalScope.servers = append(globalScope.servers, serverSpecification)
			}
			continue
		}
		scopeIndex, loaded := linkScopeIndexes[server.interfaceIndex]
		if !loaded {
			linkScope, linkLoaded, linkErr := t.loadResolvedLinkScope(ctx, managerObject, server.interfaceIndex)
			if linkErr != nil {
				return nil, linkErr
			}
			if !linkLoaded {
				linkScopeIndexes[server.interfaceIndex] = -1
				continue
			}
			scopeIndex = len(linkScopes)
			linkScopeIndexes[server.interfaceIndex] = scopeIndex
			linkScopes = append(linkScopes, linkScope)
		} else if scopeIndex < 0 {
			continue
		}
		serverSpecification, loaded := buildResolvedServerSpecification(linkScopes[scopeIndex].interfaceName, server)
		if loaded {
			linkScopes[scopeIndex].servers = append(linkScopes[scopeIndex].servers, serverSpecification)
		}
	}
	for _, domain := range managerDomains {
		domainName := mDNS.Fqdn(domain.Domain)
		if domain.InterfaceIndex == 0 {
			globalScope.domains = append(globalScope.domains, domainName)
			continue
		}
		scopeIndex, loaded := linkScopeIndexes[domain.InterfaceIndex]
		if loaded && scopeIndex >= 0 {
			linkScopes[scopeIndex].domains = append(linkScopes[scopeIndex].domains, domainName)
		}
	}
	if len(globalScope.servers) == 0 && !slices.ContainsFunc(linkScopes, func(it resolvedScopeSpecification) bool {
		return it.defaultRoute && len(it.servers) > 0
	}) {
		fallbackServers, fallbackErr := loadResolvedManagerServers(managerProperties, "FallbackDNS")
		if fallbackErr != nil {
			return nil, fallbackErr
		}
		for _, server := range fallbackServers {
			serverSpecification, loaded := buildResolvedServerSpecification(globalScope.interfaceName, server)
			if loaded {
				globalScope.servers = append(globalScope.servers, serverSpecification)
			}
		}
		globalScope.fallback = true
	}
	scopeSpecifications := common.Filter(append([]resolvedScopeSpecification{globalScope}, linkScopes...), func(it resolvedScopeSpecification) bool {
		return len(it.servers) > 0
	})
	if len(scopeSpecifications) == 0 {
		return nil, E.New("no DNS servers configured")
	}
	serverSet := &resolvedServerSet{}
	for _, scopeSpecification := range scopeSpecifications {
		scope, createErr := t.createResolvedScope(scopeSpecification)
		if createErr != nil {
			_ = serverSet.Close()
			return nil, createErr
		}
		serverSet.scopes = append(serverSet.scopes, scope)
		for _, serverSpecification := range scopeSpecification.servers {
			serverSet.serverAddresses = append(serverSet.serverAddresses, serverSpecification.address)
			serverSet.signature = append(serverSet.signature, scopeSpecification.interfaceName+"/"+M.SocksaddrFrom(serverSpecification.address, serverSpecification.port).String())
		}
	}
	return serverSet, nil
}

func (t *DBusResolvedResolver) loadResolvedLinkScope(ctx context.Context, managerObject dbus.BusObject, interfaceIndex int32) (resolvedScopeSpecification, bool, error) {
	linkInterface, err := t.interfaceFinder.ByIndex(int(interfaceIndex))
	if err != nil || slices.Contains(t.interfaceMonitor.MyInterfaces(), linkInterface.Name) {
		return resolvedScopeSpecification{}, false, nil
	}
	var linkPath dbus.ObjectPath
	err = managerObject.CallWithContext(ctx, "org.freedesktop.resolve1.Manager.GetLink", 0, interfaceIndex).Store(&linkPath)
	if err != nil {
		return resolvedScopeSpecification{}, false, err
	}
	linkProperties, err := loadResolvedProperties(ctx, t.systemBus.Object("org.freedesktop.resolve1", linkPath), "org.freedesktop.resolve1.Link")
	if err != nil {
		return resolvedScopeSpecification{}, false, err
	}
	var scopesMask uint64
	err = storeResolvedProperty(linkProperties, "ScopesMask", &scopesMask)
	if err != nil {
		return resolvedScopeSpecification{}, false, err
	}
	if scopesMask&resolvedScopesMaskDNS == 0 {
		return resolvedScopeSpecification{}, false, nil
	}
	linkScope := resolvedScopeSpecification{
		interfaceName: linkInterface.Name,
	}
	err = storeResolvedProperty(linkProperties, "DNSOverTLS", &linkScope.dnsOverTLSMode)
	if err != nil {
		return resolvedScopeSpecification{}, false, err
	}
	err = storeResolvedProperty(linkProperties, "DefaultRoute", &linkScope.defaultRoute)
	if err != nil {
		return resolvedScopeSpecification{}, false, err
	}
	return linkScope, true, nil
}

func (t *DBusResolvedResolver) createResolvedScope(scopeSpecification resolvedScopeSpecification) (resolvedScope, error) {
	scope := resolvedScope{
		domains:      scopeSpecification.domains,
		defaultRoute: scopeSpecification.defaultRoute,
		fallback:     scopeSpecification.fallback,
	}
	serverDialer, err := dialer.NewDefault(t.ctx, option.DialerOptions{
		AbstractDialerOptions: option.AbstractDialerOptions{
			BindInterface:      scopeSpecification.interfaceName,
			UDPFragmentDefault: true,
		},
	})
	if err != nil {
		return resolvedScope{}, err
	}
	for _, serverSpecification := range scopeSpecification.servers {
		server, createErr := t.createResolvedServer(serverDialer, scopeSpecification.dnsOverTLSMode, serverSpecification)
		if createErr != nil {
			_ = scope.Close()
			return resolvedScope{}, createErr
		}
		scope.servers = append(scope.servers, server)
	}
	return scope, nil
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

func buildResolvedServerSpecification(interfaceName string, server resolvedManagerServer) (resolvedServerSpecification, bool) {
	address, loaded := netip.AddrFromSlice(server.address)
	if !loaded {
		return resolvedServerSpecification{}, false
	}
	if address.Is6() && address.IsLinkLocalUnicast() && address.Zone() == "" {
		address = address.WithZone(interfaceName)
	}
	return resolvedServerSpecification{
		address:    address,
		port:       server.port,
		serverName: server.serverName,
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

func loadResolvedProperties(ctx context.Context, object dbus.BusObject, interfaceName string) (map[string]dbus.Variant, error) {
	var properties map[string]dbus.Variant
	err := object.CallWithContext(ctx, "org.freedesktop.DBus.Properties.GetAll", 0, interfaceName).Store(&properties)
	if err != nil {
		return nil, E.Cause(err, "load ", interfaceName, " properties")
	}
	return properties, nil
}

func storeResolvedProperty(properties map[string]dbus.Variant, name string, value any) error {
	property, loaded := properties[name]
	if !loaded {
		return nil
	}
	err := property.Store(value)
	if err != nil {
		return E.Cause(err, "parse resolved property ", name)
	}
	return nil
}

func loadResolvedManagerServers(properties map[string]dbus.Variant, name string) ([]resolvedManagerServer, error) {
	_, loaded := properties[name+"Ex"]
	if loaded {
		var serversEx []resolvedManagerDNSEx
		err := storeResolvedProperty(properties, name+"Ex", &serversEx)
		if err != nil {
			return nil, err
		}
		return common.Map(serversEx, func(it resolvedManagerDNSEx) resolvedManagerServer {
			return resolvedManagerServer{
				interfaceIndex: it.InterfaceIndex,
				address:        it.Address,
				port:           it.Port,
				serverName:     it.Name,
			}
		}), nil
	}
	var servers []resolvedManagerDNS
	err := storeResolvedProperty(properties, name, &servers)
	if err != nil {
		return nil, err
	}
	return common.Map(servers, func(it resolvedManagerDNS) resolvedManagerServer {
		return resolvedManagerServer{
			interfaceIndex: it.InterfaceIndex,
			address:        it.Address,
		}
	}), nil
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
	t.postUpdateStatus()
}
