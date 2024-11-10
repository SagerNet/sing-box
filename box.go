package box

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/taskmonitor"
	C "github.com/sagernet/sing-box/constant"
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
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/pause"
)

var _ adapter.Service = (*Box)(nil)

type Box struct {
	createdAt    time.Time
	router       adapter.Router
	inbound      *inbound.Manager
	outbound     *outbound.Manager
	network      *route.NetworkManager
	logFactory   log.Factory
	logger       log.ContextLogger
	preServices1 map[string]adapter.Service
	preServices2 map[string]adapter.Service
	postServices map[string]adapter.Service
	done         chan struct{}
}

type Options struct {
	option.Options
	Context           context.Context
	PlatformLogWriter log.PlatformWriter
}

func Context(ctx context.Context, inboundRegistry adapter.InboundRegistry, outboundRegistry adapter.OutboundRegistry) context.Context {
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
	return ctx
}

func New(options Options) (*Box, error) {
	createdAt := time.Now()
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = service.ContextWithDefaultRegistry(ctx)
	inboundRegistry := service.FromContext[adapter.InboundRegistry](ctx)
	if inboundRegistry == nil {
		return nil, E.New("missing inbound registry in context")
	}
	outboundRegistry := service.FromContext[adapter.OutboundRegistry](ctx)
	if outboundRegistry == nil {
		return nil, E.New("missing outbound registry in context")
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
	routeOptions := common.PtrValueOrDefault(options.Route)
	inboundManager := inbound.NewManager(logFactory.NewLogger("inbound"), inboundRegistry)
	outboundManager := outbound.NewManager(logFactory.NewLogger("outbound"), outboundRegistry, routeOptions.Final)
	ctx = service.ContextWith[adapter.InboundManager](ctx, inboundManager)
	ctx = service.ContextWith[adapter.OutboundManager](ctx, outboundManager)
	networkManager, err := route.NewNetworkManager(ctx, logFactory.NewLogger("network"), routeOptions)
	if err != nil {
		return nil, E.Cause(err, "initialize network manager")
	}
	ctx = service.ContextWith[adapter.NetworkManager](ctx, networkManager)
	router, err := route.NewRouter(ctx, logFactory, routeOptions, common.PtrValueOrDefault(options.DNS), common.PtrValueOrDefault(options.NTP))
	if err != nil {
		return nil, E.Cause(err, "initialize router")
	}
	for i, inboundOptions := range options.Inbounds {
		var tag string
		if inboundOptions.Tag != "" {
			tag = inboundOptions.Tag
		} else {
			tag = F.ToString(i)
		}
		err = inboundManager.Create(ctx,
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
	outboundManager.Initialize(common.Must1(
		direct.NewOutbound(
			ctx,
			router,
			logFactory.NewLogger("outbound/direct"),
			"direct",
			option.DirectOutboundOptions{},
		),
	))
	if platformInterface != nil {
		err = platformInterface.Initialize(networkManager)
		if err != nil {
			return nil, E.Cause(err, "initialize platform interface")
		}
	}
	preServices1 := make(map[string]adapter.Service)
	preServices2 := make(map[string]adapter.Service)
	postServices := make(map[string]adapter.Service)
	if needCacheFile {
		cacheFile := service.FromContext[adapter.CacheFile](ctx)
		if cacheFile == nil {
			cacheFile = cachefile.New(ctx, common.PtrValueOrDefault(experimentalOptions.CacheFile))
			service.MustRegister[adapter.CacheFile](ctx, cacheFile)
		}
		preServices1["cache file"] = cacheFile
	}
	if needClashAPI {
		clashAPIOptions := common.PtrValueOrDefault(experimentalOptions.ClashAPI)
		clashAPIOptions.ModeList = experimental.CalculateClashModeList(options.Options)
		clashServer, err := experimental.NewClashServer(ctx, logFactory.(log.ObservableFactory), clashAPIOptions)
		if err != nil {
			return nil, E.Cause(err, "create clash api server")
		}
		router.SetClashServer(clashServer)
		preServices2["clash api"] = clashServer
	}
	if needV2RayAPI {
		v2rayServer, err := experimental.NewV2RayServer(logFactory.NewLogger("v2ray-api"), common.PtrValueOrDefault(experimentalOptions.V2RayAPI))
		if err != nil {
			return nil, E.Cause(err, "create v2ray api server")
		}
		router.SetV2RayServer(v2rayServer)
		preServices2["v2ray api"] = v2rayServer
	}
	return &Box{
		router:       router,
		inbound:      inboundManager,
		outbound:     outboundManager,
		network:      networkManager,
		createdAt:    createdAt,
		logFactory:   logFactory,
		logger:       logFactory.Logger(),
		preServices1: preServices1,
		preServices2: preServices2,
		postServices: postServices,
		done:         make(chan struct{}),
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
	for serviceName, service := range s.preServices1 {
		if preService, isPreService := service.(adapter.LegacyPreStarter); isPreService {
			monitor.Start("pre-start ", serviceName)
			err := preService.PreStart()
			monitor.Finish()
			if err != nil {
				return E.Cause(err, "pre-start ", serviceName)
			}
		}
	}
	for serviceName, service := range s.preServices2 {
		if preService, isPreService := service.(adapter.LegacyPreStarter); isPreService {
			monitor.Start("pre-start ", serviceName)
			err := preService.PreStart()
			monitor.Finish()
			if err != nil {
				return E.Cause(err, "pre-start ", serviceName)
			}
		}
	}
	err = s.network.Start(adapter.StartStateInitialize)
	if err != nil {
		return E.Cause(err, "initialize network manager")
	}
	err = s.router.Start(adapter.StartStateInitialize)
	if err != nil {
		return E.Cause(err, "initialize router")
	}
	err = s.outbound.Start(adapter.StartStateStart)
	if err != nil {
		return err
	}
	err = s.network.Start(adapter.StartStateStart)
	if err != nil {
		return err
	}
	return s.router.Start(adapter.StartStateStart)
}

func (s *Box) start() error {
	err := s.preStart()
	if err != nil {
		return err
	}
	for serviceName, service := range s.preServices1 {
		err = service.Start()
		if err != nil {
			return E.Cause(err, "start ", serviceName)
		}
	}
	for serviceName, service := range s.preServices2 {
		err = service.Start()
		if err != nil {
			return E.Cause(err, "start ", serviceName)
		}
	}
	err = s.inbound.Start(adapter.StartStateStart)
	if err != nil {
		return err
	}
	for serviceName, service := range s.postServices {
		err := service.Start()
		if err != nil {
			return E.Cause(err, "start ", serviceName)
		}
	}
	err = s.outbound.Start(adapter.StartStatePostStart)
	if err != nil {
		return err
	}
	err = s.network.Start(adapter.StartStatePostStart)
	if err != nil {
		return err
	}
	err = s.router.Start(adapter.StartStatePostStart)
	if err != nil {
		return err
	}
	err = s.inbound.Start(adapter.StartStatePostStart)
	if err != nil {
		return err
	}
	err = s.network.Start(adapter.StartStateStarted)
	if err != nil {
		return err
	}
	err = s.router.Start(adapter.StartStateStarted)
	if err != nil {
		return err
	}
	err = s.outbound.Start(adapter.StartStateStarted)
	if err != nil {
		return err
	}
	err = s.inbound.Start(adapter.StartStateStarted)
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
	monitor := taskmonitor.New(s.logger, C.StopTimeout)
	var errors error
	for serviceName, service := range s.postServices {
		monitor.Start("close ", serviceName)
		errors = E.Append(errors, service.Close(), func(err error) error {
			return E.Cause(err, "close ", serviceName)
		})
		monitor.Finish()
	}
	errors = E.Errors(errors, s.inbound.Close())
	errors = E.Errors(errors, s.outbound.Close())
	errors = E.Errors(errors, s.network.Close())
	errors = E.Errors(errors, s.router.Close())
	for serviceName, service := range s.preServices1 {
		monitor.Start("close ", serviceName)
		errors = E.Append(errors, service.Close(), func(err error) error {
			return E.Cause(err, "close ", serviceName)
		})
		monitor.Finish()
	}
	for serviceName, service := range s.preServices2 {
		monitor.Start("close ", serviceName)
		errors = E.Append(errors, service.Close(), func(err error) error {
			return E.Cause(err, "close ", serviceName)
		})
		monitor.Finish()
	}
	if err := common.Close(s.logFactory); err != nil {
		errors = E.Append(errors, err, func(err error) error {
			return E.Cause(err, "close logger")
		})
	}
	return errors
}

func (s *Box) Inbound() adapter.InboundManager {
	return s.inbound
}

func (s *Box) Outbound() adapter.OutboundManager {
	return s.outbound
}

func (s *Box) Network() adapter.NetworkManager {
	return s.network
}

func (s *Box) Router() adapter.Router {
	return s.router
}
