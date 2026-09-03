package route

import (
	"context"
	"slices"
	"sync/atomic"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/urltest"
	"github.com/sagernet/sing-box/experimental/clashmode"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/observable"
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/pause"
)

var _ adapter.LifecycleService = (*ReferenceManager)(nil)

type ReferenceManager struct {
	ctx                    context.Context
	logger                 log.ContextLogger
	rules                  []option.Rule
	dnsRules               []option.DNSRule
	staticOutbounds        []string
	staticTransports       []string
	subscriber             *observable.Subscriber[struct{}]
	pauseManager           pause.Manager
	pauseCallback          *list.Element[pause.Callback]
	devicePaused           atomic.Bool
	idleFlushed            bool
	keepIdle               map[any]bool
	unreferencedTransports map[string]bool
}

func NewReferenceManager(ctx context.Context, logger log.ContextLogger, options option.Options) *ReferenceManager {
	var (
		staticOutbounds  []string
		staticTransports []string
	)
	if options.NTP != nil && options.NTP.Enabled && options.NTP.Detour != "" {
		staticOutbounds = append(staticOutbounds, options.NTP.Detour)
	}
	for _, outboundOptions := range options.Outbounds {
		staticTransports = appendDomainResolver(staticTransports, outboundOptions.Options)
	}
	for _, endpointOptions := range options.Endpoints {
		staticTransports = appendDomainResolver(staticTransports, endpointOptions.Options)
	}
	var (
		rules    []option.Rule
		dnsRules []option.DNSRule
	)
	if options.Route != nil {
		rules = options.Route.Rules
	}
	if options.DNS != nil {
		dnsRules = options.DNS.Rules
	}
	return &ReferenceManager{
		ctx:              ctx,
		logger:           logger,
		rules:            rules,
		dnsRules:         dnsRules,
		staticOutbounds:  staticOutbounds,
		staticTransports: staticTransports,
		pauseManager:     service.FromContext[pause.Manager](ctx),
	}
}

func appendDomainResolver(transports []string, rawOptions any) []string {
	dialerOptionsWrapper, isDialerOptionsWrapper := rawOptions.(option.DialerOptionsWrapper)
	if !isDialerOptionsWrapper {
		return transports
	}
	dialerOptions := dialerOptionsWrapper.TakeDialerOptions()
	if dialerOptions.DomainResolver == nil || dialerOptions.DomainResolver.Server == "" {
		return transports
	}
	return append(transports, dialerOptions.DomainResolver.Server)
}

func (m *ReferenceManager) Name() string {
	return "reference manager"
}

func (m *ReferenceManager) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStarted {
		return nil
	}
	m.subscriber = observable.NewSubscriber[struct{}](1)
	history := service.PtrFromContext[urltest.HistoryStorage](m.ctx)
	if history != nil {
		history.AddUpdateHook(m.subscriber)
	}
	clashMode := service.PtrFromContext[clashmode.Manager](m.ctx)
	if clashMode != nil {
		clashMode.AddUpdateHook(m.subscriber)
	}
	if m.pauseManager != nil {
		m.devicePaused.Store(m.pauseManager.IsDevicePaused())
	}
	m.update()
	go m.loop()
	if m.pauseManager != nil {
		m.pauseCallback = m.pauseManager.RegisterCallback(func(event int) {
			switch event {
			case pause.EventDevicePaused:
				m.devicePaused.Store(true)
			case pause.EventDeviceWake:
				m.devicePaused.Store(false)
			default:
				return
			}
			m.subscriber.Emit(struct{}{})
		})
	}
	return nil
}

func (m *ReferenceManager) Close() error {
	if m.pauseCallback != nil {
		m.pauseManager.UnregisterCallback(m.pauseCallback)
		m.pauseCallback = nil
	}
	if m.subscriber != nil {
		m.subscriber.Close()
	}
	return nil
}

func (m *ReferenceManager) loop() {
	subscription, done := m.subscriber.Subscription()
	for {
		select {
		case <-subscription:
		case <-done:
			return
		}
		m.update()
	}
}

func (m *ReferenceManager) update() {
	var mode string
	clashMode := service.PtrFromContext[clashmode.Manager](m.ctx)
	if clashMode != nil {
		mode = clashMode.Mode()
	}
	outboundManager := service.FromContext[adapter.OutboundManager](m.ctx)
	endpointManager := service.FromContext[adapter.EndpointManager](m.ctx)
	inboundManager := service.FromContext[adapter.InboundManager](m.ctx)
	serviceManager := service.FromContext[adapter.ServiceManager](m.ctx)
	transportManager := service.FromContext[adapter.DNSTransportManager](m.ctx)
	networkManager := service.FromContext[adapter.NetworkManager](m.ctx)
	httpClientManager := service.FromContext[adapter.HTTPClientManager](m.ctx)

	transportQueue := slices.Clone(m.staticTransports)
	outboundQueue := slices.Clone(m.staticOutbounds)
	if !collectDNSRuleReferences(m.dnsRules, mode, &transportQueue) {
		defaultTransport := transportManager.Default()
		if defaultTransport != nil {
			transportQueue = append(transportQueue, defaultTransport.Tag())
		}
	}
	transportQueue = append(transportQueue, networkManager.DefaultOptions().DomainResolver)
	if !collectRuleReferences(m.rules, mode, &outboundQueue, &transportQueue) {
		defaultOutbound := outboundManager.Default()
		if defaultOutbound != nil {
			outboundQueue = append(outboundQueue, defaultOutbound.Tag())
		}
	}
	var onDemandEndpoints []adapter.OnDemandEndpoint
	for _, endpoint := range endpointManager.Endpoints() {
		onDemandEndpoint, isOnDemandEndpoint := endpoint.(adapter.OnDemandEndpoint)
		if isOnDemandEndpoint && onDemandEndpoint.OnDemand() {
			onDemandEndpoints = append(onDemandEndpoints, onDemandEndpoint)
			continue
		}
		outboundQueue = append(outboundQueue, endpoint.Tag())
	}
	for _, inbound := range inboundManager.Inbounds() {
		referrer, isReferrer := inbound.(adapter.Referrer)
		if isReferrer {
			outboundQueue = append(outboundQueue, referrer.References()...)
		}
	}
	for _, boxService := range serviceManager.Services() {
		referrer, isReferrer := boxService.(adapter.Referrer)
		if isReferrer {
			outboundQueue = append(outboundQueue, referrer.References()...)
		}
	}
	referrer, isReferrer := httpClientManager.(adapter.Referrer)
	if isReferrer {
		outboundQueue = append(outboundQueue, referrer.References()...)
	}

	referencedTransports := make(map[string]bool)
	for len(transportQueue) > 0 {
		tag := transportQueue[0]
		transportQueue = transportQueue[1:]
		if tag == "" || referencedTransports[tag] {
			continue
		}
		transport, loaded := transportManager.Transport(tag)
		if !loaded {
			continue
		}
		referencedTransports[tag] = true
		transportQueue = append(transportQueue, transport.Dependencies()...)
		transportReferrer, isTransportReferrer := transport.(adapter.Referrer)
		if isTransportReferrer {
			outboundQueue = append(outboundQueue, transportReferrer.References()...)
		}
	}
	referencedOutbounds := make(map[string]bool)
	for len(outboundQueue) > 0 {
		tag := outboundQueue[0]
		outboundQueue = outboundQueue[1:]
		if tag == "" || referencedOutbounds[tag] {
			continue
		}
		outbound, loaded := outboundManager.Outbound(tag)
		if !loaded {
			continue
		}
		referencedOutbounds[tag] = true
		outboundReferrer, isOutboundReferrer := outbound.(adapter.Referrer)
		if isOutboundReferrer {
			outboundQueue = append(outboundQueue, outboundReferrer.References()...)
		} else {
			outboundQueue = append(outboundQueue, outbound.Dependencies()...)
		}
	}

	devicePaused := m.devicePaused.Load()
	keepIdle := make(map[any]bool)
	for _, outbound := range outboundManager.Outbounds() {
		keeper, isKeeper := outbound.(adapter.IdleConnectionKeeper)
		if !isKeeper {
			continue
		}
		m.applyKeepIdle(keepIdle, idleTarget{
			value:    outbound,
			keeper:   keeper,
			kind:     "outbound/",
			action:   "closing idle connections",
			typeName: outbound.Type(),
			tag:      outbound.Tag(),
			keep:     referencedOutbounds[outbound.Tag()],
		})
	}
	for _, endpoint := range onDemandEndpoints {
		m.applyKeepIdle(keepIdle, idleTarget{
			value:        endpoint,
			keeper:       endpoint,
			kind:         "endpoint/",
			action:       "suspending",
			typeName:     endpoint.Type(),
			tag:          endpoint.Tag(),
			keep:         referencedOutbounds[endpoint.Tag()] && !devicePaused,
			devicePaused: devicePaused,
		})
	}
	unreferencedTransports := make(map[string]bool)
	for _, transport := range transportManager.Transports() {
		tag := transport.Tag()
		referenced := referencedTransports[tag]
		keeper, isKeeper := transport.(adapter.IdleConnectionKeeper)
		if isKeeper {
			m.applyKeepIdle(keepIdle, idleTarget{
				value:    transport,
				keeper:   keeper,
				kind:     "dns/",
				action:   "closing idle connections",
				typeName: transport.Type(),
				tag:      tag,
				keep:     referenced,
			})
			continue
		}
		if referenced {
			continue
		}
		unreferencedTransports[tag] = true
		if m.unreferencedTransports != nil && !m.unreferencedTransports[tag] {
			m.logger.Debug("dns/", transport.Type(), "[", tag, "] is unreferenced, resetting")
			transport.Reset()
		}
	}
	m.keepIdle = keepIdle
	m.unreferencedTransports = unreferencedTransports
	if devicePaused && !m.idleFlushed {
		m.CloseIdleConnections()
	}
	m.idleFlushed = devicePaused
}

type idleKeeper interface {
	SetKeepIdleConnections(keep bool)
}

type idleTarget struct {
	value        any
	keeper       idleKeeper
	kind         string
	action       string
	typeName     string
	tag          string
	keep         bool
	devicePaused bool
}

func (m *ReferenceManager) applyKeepIdle(keepIdle map[any]bool, target idleTarget) {
	keepIdle[target.value] = target.keep
	previous, tracked := m.keepIdle[target.value]
	if tracked && previous == target.keep {
		return
	}
	if !target.keep {
		if target.devicePaused {
			m.logger.Debug(target.kind, target.typeName, "[", target.tag, "] device paused, ", target.action)
		} else {
			m.logger.Debug(target.kind, target.typeName, "[", target.tag, "] is unreferenced, ", target.action)
		}
	}
	target.keeper.SetKeepIdleConnections(target.keep)
}

func (m *ReferenceManager) CloseIdleConnections() {
	outboundManager := service.FromContext[adapter.OutboundManager](m.ctx)
	transportManager := service.FromContext[adapter.DNSTransportManager](m.ctx)
	for _, outbound := range outboundManager.Outbounds() {
		keeper, isKeeper := outbound.(adapter.IdleConnectionKeeper)
		if isKeeper {
			keeper.CloseIdleConnections()
		}
	}
	for _, transport := range transportManager.Transports() {
		keeper, isKeeper := transport.(adapter.IdleConnectionKeeper)
		if isKeeper {
			keeper.CloseIdleConnections()
		}
	}
}
