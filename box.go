package box

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/endpoint"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/adapter/outbound"
	boxService "github.com/sagernet/sing-box/adapter/service"
	"github.com/sagernet/sing-box/common/certificate"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/taskmonitor"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/dns/transport/local"
	"github.com/sagernet/sing-box/experimental"
	"github.com/sagernet/sing-box/experimental/cachefile"
	"github.com/sagernet/sing-box/experimental/libbox/platform"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/protocol/direct"
	"github.com/sagernet/sing-box/route"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/ntp"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/pause"
)

var _ adapter.SimpleLifecycle = (*Box)(nil)

type Box struct {
	createdAt       time.Time
	logFactory      log.Factory
	logger          log.ContextLogger
	network         *route.NetworkManager
	endpoint        *endpoint.Manager
	inbound         *inbound.Manager
	outbound        *outbound.Manager
	service         *boxService.Manager
	dnsTransport    *dns.TransportManager
	dnsRouter       *dns.Router
	connection      *route.ConnectionManager
	router          *route.Router
	internalService []adapter.LifecycleService
	done            chan struct{}
	currentOptions  option.Options
	ctx             context.Context
}

type Options struct {
	option.Options
	Context           context.Context
	PlatformLogWriter log.PlatformWriter
}

func Context(
	ctx context.Context,
	inboundRegistry adapter.InboundRegistry,
	outboundRegistry adapter.OutboundRegistry,
	endpointRegistry adapter.EndpointRegistry,
	dnsTransportRegistry adapter.DNSTransportRegistry,
	serviceRegistry adapter.ServiceRegistry,
) context.Context {
	if service.FromContext[option.InboundOptionsRegistry](ctx) == nil ||
		service.FromContext[adapter.InboundRegistry](ctx) == nil {
		ctx = service.ContextWith[option.InboundOptionsRegistry](ctx, inboundRegistry)
		ctx = service.ContextWith[adapter.InboundRegistry](ctx, inboundRegistry)
	}
	if service.FromContext[option.OutboundOptionsRegistry](ctx) == nil ||
		service.FromContext[adapter.OutboundRegistry](ctx) == nil {
		ctx = service.ContextWith[option.OutboundOptionsRegistry](ctx, outboundRegistry)
		ctx = service.ContextWith[adapter.OutboundRegistry](ctx, outboundRegistry)
	}
	if service.FromContext[option.EndpointOptionsRegistry](ctx) == nil ||
		service.FromContext[adapter.EndpointRegistry](ctx) == nil {
		ctx = service.ContextWith[option.EndpointOptionsRegistry](ctx, endpointRegistry)
		ctx = service.ContextWith[adapter.EndpointRegistry](ctx, endpointRegistry)
	}
	if service.FromContext[adapter.DNSTransportRegistry](ctx) == nil {
		ctx = service.ContextWith[option.DNSTransportOptionsRegistry](ctx, dnsTransportRegistry)
		ctx = service.ContextWith[adapter.DNSTransportRegistry](ctx, dnsTransportRegistry)
	}
	if service.FromContext[adapter.ServiceRegistry](ctx) == nil {
		ctx = service.ContextWith[option.ServiceOptionsRegistry](ctx, serviceRegistry)
		ctx = service.ContextWith[adapter.ServiceRegistry](ctx, serviceRegistry)
	}
	return ctx
}

func New(options Options) (*Box, error) {
	createdAt := time.Now()
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = service.ContextWithDefaultRegistry(ctx)

	endpointRegistry := service.FromContext[adapter.EndpointRegistry](ctx)
	inboundRegistry := service.FromContext[adapter.InboundRegistry](ctx)
	outboundRegistry := service.FromContext[adapter.OutboundRegistry](ctx)
	dnsTransportRegistry := service.FromContext[adapter.DNSTransportRegistry](ctx)
	serviceRegistry := service.FromContext[adapter.ServiceRegistry](ctx)

	if endpointRegistry == nil {
		return nil, E.New("missing endpoint registry in context")
	}
	if inboundRegistry == nil {
		return nil, E.New("missing inbound registry in context")
	}
	if outboundRegistry == nil {
		return nil, E.New("missing outbound registry in context")
	}
	if dnsTransportRegistry == nil {
		return nil, E.New("missing DNS transport registry in context")
	}
	if serviceRegistry == nil {
		return nil, E.New("missing service registry in context")
	}

	ctx = pause.WithDefaultManager(ctx)
	experimentalOptions := common.PtrValueOrDefault(options.Experimental)
	applyDebugOptions(common.PtrValueOrDefault(experimentalOptions.Debug))
	var needCacheFile bool
	var needClashAPI bool
	var needV2RayAPI bool
	if experimentalOptions.CacheFile != nil && experimentalOptions.CacheFile.Enabled || options.PlatformLogWriter != nil {
		needCacheFile = true
	}
	if experimentalOptions.ClashAPI != nil || options.PlatformLogWriter != nil {
		needClashAPI = true
	}
	if experimentalOptions.V2RayAPI != nil && experimentalOptions.V2RayAPI.Listen != "" {
		needV2RayAPI = true
	}
	platformInterface := service.FromContext[platform.Interface](ctx)
	var defaultLogWriter io.Writer
	if platformInterface != nil {
		defaultLogWriter = io.Discard
	}
	logFactory, err := log.New(log.Options{
		Context:        ctx,
		Options:        common.PtrValueOrDefault(options.Log),
		Observable:     needClashAPI,
		DefaultWriter:  defaultLogWriter,
		BaseTime:       createdAt,
		PlatformWriter: options.PlatformLogWriter,
	})
	if err != nil {
		return nil, E.Cause(err, "create log factory")
	}

	var internalServices []adapter.LifecycleService
	certificateOptions := common.PtrValueOrDefault(options.Certificate)
	if C.IsAndroid || certificateOptions.Store != "" && certificateOptions.Store != C.CertificateStoreSystem ||
		len(certificateOptions.Certificate) > 0 ||
		len(certificateOptions.CertificatePath) > 0 ||
		len(certificateOptions.CertificateDirectoryPath) > 0 {
		certificateStore, err := certificate.NewStore(ctx, logFactory.NewLogger("certificate"), certificateOptions)
		if err != nil {
			return nil, err
		}
		service.MustRegister[adapter.CertificateStore](ctx, certificateStore)
		internalServices = append(internalServices, certificateStore)
	}

	routeOptions := common.PtrValueOrDefault(options.Route)
	dnsOptions := common.PtrValueOrDefault(options.DNS)
	endpointManager := endpoint.NewManager(logFactory.NewLogger("endpoint"), endpointRegistry)
	inboundManager := inbound.NewManager(logFactory.NewLogger("inbound"), inboundRegistry, endpointManager)
	outboundManager := outbound.NewManager(logFactory.NewLogger("outbound"), outboundRegistry, endpointManager, routeOptions.Final)
	dnsTransportManager := dns.NewTransportManager(logFactory.NewLogger("dns/transport"), dnsTransportRegistry, outboundManager, dnsOptions.Final)
	serviceManager := boxService.NewManager(logFactory.NewLogger("service"), serviceRegistry)
	service.MustRegister[adapter.EndpointManager](ctx, endpointManager)
	service.MustRegister[adapter.InboundManager](ctx, inboundManager)
	service.MustRegister[adapter.OutboundManager](ctx, outboundManager)
	service.MustRegister[adapter.DNSTransportManager](ctx, dnsTransportManager)
	service.MustRegister[adapter.ServiceManager](ctx, serviceManager)
	dnsRouter := dns.NewRouter(ctx, logFactory, dnsOptions)
	service.MustRegister[adapter.DNSRouter](ctx, dnsRouter)
	networkManager, err := route.NewNetworkManager(ctx, logFactory.NewLogger("network"), routeOptions)
	if err != nil {
		return nil, E.Cause(err, "initialize network manager")
	}
	service.MustRegister[adapter.NetworkManager](ctx, networkManager)
	connectionManager := route.NewConnectionManager(logFactory.NewLogger("connection"))
	service.MustRegister[adapter.ConnectionManager](ctx, connectionManager)
	router := route.NewRouter(ctx, logFactory, routeOptions, dnsOptions)
	service.MustRegister[adapter.Router](ctx, router)
	err = router.Initialize(routeOptions.Rules, routeOptions.RuleSet)
	if err != nil {
		return nil, E.Cause(err, "initialize router")
	}
	ntpOptions := common.PtrValueOrDefault(options.NTP)
	var timeService *tls.TimeServiceWrapper
	if ntpOptions.Enabled {
		timeService = new(tls.TimeServiceWrapper)
		service.MustRegister[ntp.TimeService](ctx, timeService)
	}
	for i, transportOptions := range dnsOptions.Servers {
		var tag string
		if transportOptions.Tag != "" {
			tag = transportOptions.Tag
		} else {
			tag = F.ToString(i)
		}
		err = dnsTransportManager.Create(
			ctx,
			logFactory.NewLogger(F.ToString("dns/", transportOptions.Type, "[", tag, "]")),
			tag,
			transportOptions.Type,
			transportOptions.Options,
		)
		if err != nil {
			return nil, E.Cause(err, "initialize DNS server[", i, "]")
		}
	}
	err = dnsRouter.Initialize(dnsOptions.Rules)
	if err != nil {
		return nil, E.Cause(err, "initialize dns router")
	}
	for i, endpointOptions := range options.Endpoints {
		var tag string
		if endpointOptions.Tag != "" {
			tag = endpointOptions.Tag
		} else {
			tag = F.ToString(i)
		}
		endpointCtx := ctx
		if tag != "" {
			// TODO: remove this
			endpointCtx = adapter.WithContext(endpointCtx, &adapter.InboundContext{
				Outbound: tag,
			})
		}
		err = endpointManager.Create(
			endpointCtx,
			router,
			logFactory.NewLogger(F.ToString("endpoint/", endpointOptions.Type, "[", tag, "]")),
			tag,
			endpointOptions.Type,
			endpointOptions.Options,
		)
		if err != nil {
			return nil, E.Cause(err, "initialize endpoint[", i, "]")
		}
	}
	for i, inboundOptions := range options.Inbounds {
		var tag string
		if inboundOptions.Tag != "" {
			tag = inboundOptions.Tag
		} else {
			tag = F.ToString(i)
		}
		err = inboundManager.Create(
			ctx,
			router,
			logFactory.NewLogger(F.ToString("inbound/", inboundOptions.Type, "[", tag, "]")),
			tag,
			inboundOptions.Type,
			inboundOptions.Options,
		)
		if err != nil {
			return nil, E.Cause(err, "initialize inbound[", i, "]")
		}
	}
	for i, outboundOptions := range options.Outbounds {
		var tag string
		if outboundOptions.Tag != "" {
			tag = outboundOptions.Tag
		} else {
			tag = F.ToString(i)
		}
		outboundCtx := ctx
		if tag != "" {
			// TODO: remove this
			outboundCtx = adapter.WithContext(outboundCtx, &adapter.InboundContext{
				Outbound: tag,
			})
		}
		err = outboundManager.Create(
			outboundCtx,
			router,
			logFactory.NewLogger(F.ToString("outbound/", outboundOptions.Type, "[", tag, "]")),
			tag,
			outboundOptions.Type,
			outboundOptions.Options,
		)
		if err != nil {
			return nil, E.Cause(err, "initialize outbound[", i, "]")
		}
	}
	for i, serviceOptions := range options.Services {
		var tag string
		if serviceOptions.Tag != "" {
			tag = serviceOptions.Tag
		} else {
			tag = F.ToString(i)
		}
		err = serviceManager.Create(
			ctx,
			logFactory.NewLogger(F.ToString("service/", serviceOptions.Type, "[", tag, "]")),
			tag,
			serviceOptions.Type,
			serviceOptions.Options,
		)
		if err != nil {
			return nil, E.Cause(err, "initialize service[", i, "]")
		}
	}
	outboundManager.Initialize(func() (adapter.Outbound, error) {
		return direct.NewOutbound(
			ctx,
			router,
			logFactory.NewLogger("outbound/direct"),
			"direct",
			option.DirectOutboundOptions{},
		)
	})
	dnsTransportManager.Initialize(func() (adapter.DNSTransport, error) {
		return local.NewTransport(
			ctx,
			logFactory.NewLogger("dns/local"),
			"local",
			option.LocalDNSServerOptions{},
		)
	})
	if platformInterface != nil {
		err = platformInterface.Initialize(networkManager)
		if err != nil {
			return nil, E.Cause(err, "initialize platform interface")
		}
	}
	if needCacheFile {
		cacheFile := cachefile.New(ctx, common.PtrValueOrDefault(experimentalOptions.CacheFile))
		service.MustRegister[adapter.CacheFile](ctx, cacheFile)
		internalServices = append(internalServices, cacheFile)
	}
	if needClashAPI {
		clashAPIOptions := common.PtrValueOrDefault(experimentalOptions.ClashAPI)
		clashAPIOptions.ModeList = experimental.CalculateClashModeList(options.Options)
		clashServer, err := experimental.NewClashServer(ctx, logFactory.(log.ObservableFactory), clashAPIOptions)
		if err != nil {
			return nil, E.Cause(err, "create clash-server")
		}
		router.AppendTracker(clashServer)
		service.MustRegister[adapter.ClashServer](ctx, clashServer)
		internalServices = append(internalServices, clashServer)
	}
	if needV2RayAPI {
		v2rayServer, err := experimental.NewV2RayServer(logFactory.NewLogger("v2ray-api"), common.PtrValueOrDefault(experimentalOptions.V2RayAPI))
		if err != nil {
			return nil, E.Cause(err, "create v2ray-server")
		}
		if v2rayServer.StatsService() != nil {
			router.AppendTracker(v2rayServer.StatsService())
			internalServices = append(internalServices, v2rayServer)
			service.MustRegister[adapter.V2RayServer](ctx, v2rayServer)
		}
	}
	if ntpOptions.Enabled {
		ntpDialer, err := dialer.New(ctx, ntpOptions.DialerOptions, ntpOptions.ServerIsDomain())
		if err != nil {
			return nil, E.Cause(err, "create NTP service")
		}
		ntpService := ntp.NewService(ntp.Options{
			Context:       ctx,
			Dialer:        ntpDialer,
			Logger:        logFactory.NewLogger("ntp"),
			Server:        ntpOptions.ServerOptions.Build(),
			Interval:      time.Duration(ntpOptions.Interval),
			WriteToSystem: ntpOptions.WriteToSystem,
		})
		timeService.TimeService = ntpService
		internalServices = append(internalServices, adapter.NewLifecycleService(ntpService, "ntp service"))
	}
	return &Box{
		network:         networkManager,
		endpoint:        endpointManager,
		inbound:         inboundManager,
		outbound:        outboundManager,
		dnsTransport:    dnsTransportManager,
		service:         serviceManager,
		dnsRouter:       dnsRouter,
		connection:      connectionManager,
		router:          router,
		createdAt:       createdAt,
		logFactory:      logFactory,
		logger:          logFactory.Logger(),
		internalService: internalServices,
		done:            make(chan struct{}),
		currentOptions:  options.Options,
		ctx:             ctx,
	}, nil
}

func (s *Box) PreStart() error {
	err := s.preStart()
	if err != nil {
		// TODO: remove catch error
		defer func() {
			v := recover()
			if v != nil {
				println(err.Error())
				debug.PrintStack()
				panic("panic on early close: " + fmt.Sprint(v))
			}
		}()
		s.Close()
		return err
	}
	s.logger.Info("sing-box pre-started (", F.Seconds(time.Since(s.createdAt).Seconds()), "s)")
	return nil
}

func (s *Box) Start() error {
	err := s.start()
	if err != nil {
		// TODO: remove catch error
		defer func() {
			v := recover()
			if v != nil {
				println(err.Error())
				debug.PrintStack()
				println("panic on early start: " + fmt.Sprint(v))
			}
		}()
		s.Close()
		return err
	}
	s.logger.Info("sing-box started (", F.Seconds(time.Since(s.createdAt).Seconds()), "s)")
	return nil
}

func (s *Box) preStart() error {
	monitor := taskmonitor.New(s.logger, C.StartTimeout)
	monitor.Start("start logger")
	err := s.logFactory.Start()
	monitor.Finish()
	if err != nil {
		return E.Cause(err, "start logger")
	}
	err = adapter.StartNamed(adapter.StartStateInitialize, s.internalService) // cache-file clash-api v2ray-api
	if err != nil {
		return err
	}
	err = adapter.Start(adapter.StartStateInitialize, s.network, s.dnsTransport, s.dnsRouter, s.connection, s.router, s.outbound, s.inbound, s.endpoint, s.service)
	if err != nil {
		return err
	}
	err = adapter.Start(adapter.StartStateStart, s.outbound, s.dnsTransport, s.dnsRouter, s.network, s.connection, s.router)
	if err != nil {
		return err
	}
	return nil
}

func (s *Box) start() error {
	err := s.preStart()
	if err != nil {
		return err
	}
	err = adapter.StartNamed(adapter.StartStateStart, s.internalService)
	if err != nil {
		return err
	}
	err = adapter.Start(adapter.StartStateStart, s.inbound, s.endpoint, s.service)
	if err != nil {
		return err
	}
	err = adapter.Start(adapter.StartStatePostStart, s.outbound, s.network, s.dnsTransport, s.dnsRouter, s.connection, s.router, s.inbound, s.endpoint, s.service)
	if err != nil {
		return err
	}
	err = adapter.StartNamed(adapter.StartStatePostStart, s.internalService)
	if err != nil {
		return err
	}
	err = adapter.Start(adapter.StartStateStarted, s.network, s.dnsTransport, s.dnsRouter, s.connection, s.router, s.outbound, s.inbound, s.endpoint, s.service)
	if err != nil {
		return err
	}
	err = adapter.StartNamed(adapter.StartStateStarted, s.internalService)
	if err != nil {
		return err
	}
	return nil
}

func (s *Box) Close() error {
	select {
	case <-s.done:
		return os.ErrClosed
	default:
		close(s.done)
	}
	err := common.Close(
		s.service, s.endpoint, s.inbound, s.outbound, s.router, s.connection, s.dnsRouter, s.dnsTransport, s.network,
	)
	for _, lifecycleService := range s.internalService {
		err = E.Append(err, lifecycleService.Close(), func(err error) error {
			return E.Cause(err, "close ", lifecycleService.Name())
		})
	}
	err = E.Append(err, s.logFactory.Close(), func(err error) error {
		return E.Cause(err, "close logger")
	})
	return err
}

func (s *Box) Network() adapter.NetworkManager {
	return s.network
}

func (s *Box) Router() adapter.Router {
	return s.router
}

func (s *Box) Inbound() adapter.InboundManager {
	return s.inbound
}

func (s *Box) Outbound() adapter.OutboundManager {
	return s.outbound
}

func (s *Box) Reload(newOptions option.Options) error {
	s.logger.Info("reloading configuration...")

	// Reload endpoints
	oldEndpointMap := make(map[string]option.Endpoint)
	for _, ep := range s.currentOptions.Endpoints {
		oldEndpointMap[ep.Tag] = ep
	}

	newEndpointMap := make(map[string]option.Endpoint)
	for _, ep := range newOptions.Endpoints {
		newEndpointMap[ep.Tag] = ep
	}

	// Process endpoint changes
	for tag, newEp := range newEndpointMap {
		oldEp, exists := oldEndpointMap[tag]
		if !exists {
			// New endpoint - create it
			s.logger.Info("creating new endpoint: ", tag)
			err := s.endpoint.Create(
				s.ctx,
				s.router,
				s.logFactory.NewLogger(F.ToString("endpoint/", newEp.Type, "[", tag, "]")),
				tag,
				newEp.Type,
				newEp.Options,
			)
			if err != nil {
				return E.Cause(err, "create endpoint[", tag, "]")
			}
		} else if !endpointsEqual(oldEp, newEp) {
			// Modified endpoint - try to reload
			s.logger.Info("reloading endpoint: ", tag)
			err := s.endpoint.Reload(tag, newEp.Options)
			if err != nil {
				s.logger.Warn("endpoint ", tag, " does not support reload, recreating: ", err)
				// Fall back to recreate
				err = s.endpoint.Create(
					s.ctx,
					s.router,
					s.logFactory.NewLogger(F.ToString("endpoint/", newEp.Type, "[", tag, "]")),
					tag,
					newEp.Type,
					newEp.Options,
				)
				if err != nil {
					return E.Cause(err, "recreate endpoint[", tag, "]")
				}
			}
		}
	}

	// Remove deleted endpoints
	for tag := range oldEndpointMap {
		if _, exists := newEndpointMap[tag]; !exists {
			s.logger.Info("removing endpoint: ", tag)
			err := s.endpoint.Remove(tag)
			if err != nil {
				return E.Cause(err, "remove endpoint[", tag, "]")
			}
		}
	}

	// Reload inbounds
	oldInboundMap := make(map[string]option.Inbound)
	for _, ib := range s.currentOptions.Inbounds {
		oldInboundMap[ib.Tag] = ib
	}

	newInboundMap := make(map[string]option.Inbound)
	for _, ib := range newOptions.Inbounds {
		newInboundMap[ib.Tag] = ib
	}

	// Process inbound changes
	for tag, newIb := range newInboundMap {
		oldIb, exists := oldInboundMap[tag]
		if !exists {
			// New inbound - create it
			s.logger.Info("creating new inbound: ", tag)
			err := s.inbound.Create(
				s.ctx,
				s.router,
				s.logFactory.NewLogger(F.ToString("inbound/", newIb.Type, "[", tag, "]")),
				tag,
				newIb.Type,
				newIb.Options,
			)
			if err != nil {
				return E.Cause(err, "create inbound[", tag, "]")
			}
		} else if !inboundsEqual(oldIb, newIb) {
			// Modified inbound - try to reload
			s.logger.Info("reloading inbound: ", tag)
			err := s.inbound.Reload(tag, newIb.Options)
			if err != nil {
				s.logger.Warn("inbound ", tag, " does not support reload, recreating: ", err)
				// Fall back to recreate
				err = s.inbound.Create(
					s.ctx,
					s.router,
					s.logFactory.NewLogger(F.ToString("inbound/", newIb.Type, "[", tag, "]")),
					tag,
					newIb.Type,
					newIb.Options,
				)
				if err != nil {
					return E.Cause(err, "recreate inbound[", tag, "]")
				}
			}
		}
	}

	// Remove deleted inbounds
	for tag := range oldInboundMap {
		if _, exists := newInboundMap[tag]; !exists {
			s.logger.Info("removing inbound: ", tag)
			err := s.inbound.Remove(tag)
			if err != nil {
				return E.Cause(err, "remove inbound[", tag, "]")
			}
		}
	}

	// Reload outbounds
	oldOutboundMap := make(map[string]option.Outbound)
	for _, ob := range s.currentOptions.Outbounds {
		oldOutboundMap[ob.Tag] = ob
	}

	newOutboundMap := make(map[string]option.Outbound)
	for _, ob := range newOptions.Outbounds {
		newOutboundMap[ob.Tag] = ob
	}

	// Process outbound changes
	for tag, newOb := range newOutboundMap {
		oldOb, exists := oldOutboundMap[tag]
		if !exists {
			// New outbound - create it
			s.logger.Info("creating new outbound: ", tag)
			outboundCtx := s.ctx
			if tag != "" {
				outboundCtx = adapter.WithContext(outboundCtx, &adapter.InboundContext{
					Outbound: tag,
				})
			}
			err := s.outbound.Create(
				outboundCtx,
				s.router,
				s.logFactory.NewLogger(F.ToString("outbound/", newOb.Type, "[", tag, "]")),
				tag,
				newOb.Type,
				newOb.Options,
			)
			if err != nil {
				return E.Cause(err, "create outbound[", tag, "]")
			}
		} else if !outboundsEqual(oldOb, newOb) {
			// Modified outbound - try to reload
			s.logger.Info("reloading outbound: ", tag)
			err := s.outbound.Reload(tag, newOb.Options)
			if err != nil {
				s.logger.Warn("outbound ", tag, " does not support reload, recreating: ", err)
				// Fall back to recreate
				outboundCtx := s.ctx
				if tag != "" {
					outboundCtx = adapter.WithContext(outboundCtx, &adapter.InboundContext{
						Outbound: tag,
					})
				}
				err = s.outbound.Create(
					outboundCtx,
					s.router,
					s.logFactory.NewLogger(F.ToString("outbound/", newOb.Type, "[", tag, "]")),
					tag,
					newOb.Type,
					newOb.Options,
				)
				if err != nil {
					return E.Cause(err, "recreate outbound[", tag, "]")
				}
			}
		}
	}

	// Remove deleted outbounds
	for tag := range oldOutboundMap {
		if _, exists := newOutboundMap[tag]; !exists {
			s.logger.Info("removing outbound: ", tag)
			err := s.outbound.Remove(tag)
			if err != nil {
				return E.Cause(err, "remove outbound[", tag, "]")
			}
		}
	}

	// Update current options
	s.currentOptions = newOptions

	s.logger.Info("configuration reloaded successfully")
	return nil
}

// Helper functions for comparing configurations
func endpointsEqual(a, b option.Endpoint) bool {
	// Simple JSON comparison for now
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	return string(aJSON) == string(bJSON)
}

func inboundsEqual(a, b option.Inbound) bool {
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	return string(aJSON) == string(bJSON)
}

func outboundsEqual(a, b option.Outbound) bool {
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	return string(aJSON) == string(bJSON)
}
