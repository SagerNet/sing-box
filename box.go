package box

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"time"

	"github.com/sagernet/sing-box/adapter"
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
	inbounds     []adapter.Inbound
	outbounds    []adapter.Outbound
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
	inboundRegistry := service.FromContext[adapter.InboundRegistry](ctx)
	if inboundRegistry == nil {
		return nil, E.New("missing inbound registry in context")
	}
	outboundRegistry := service.FromContext[adapter.OutboundRegistry](ctx)
	if outboundRegistry == nil {
		return nil, E.New("missing outbound registry in context")
	}
	ctx = service.ContextWithDefaultRegistry(ctx)
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
	router, err := route.NewRouter(
		ctx,
		logFactory,
		common.PtrValueOrDefault(options.Route),
		common.PtrValueOrDefault(options.DNS),
		common.PtrValueOrDefault(options.NTP),
		options.Inbounds,
	)
	if err != nil {
		return nil, E.Cause(err, "parse route options")
	}
	for i, inboundOptions := range options.Inbounds {
		var currentInbound adapter.Inbound
		var tag string
		if inboundOptions.Tag != "" {
			tag = inboundOptions.Tag
		} else {
			tag = F.ToString(i)
		}
		currentInbound, err = inboundRegistry.CreateInbound(
			ctx,
			router,
			logFactory.NewLogger(F.ToString("inbound/", inboundOptions.Type, "[", tag, "]")),
			tag,
			inboundOptions.Type,
			inboundOptions.Options,
		)
		if err != nil {
			return nil, E.Cause(err, "parse inbound[", i, "]")
		}
		inbounds = append(inbounds, currentInbound)
	}
	for i, outboundOptions := range options.Outbounds {
		var currentOutbound adapter.Outbound
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
		currentOutbound, err = outboundRegistry.CreateOutbound(
			outboundCtx,
			router,
			logFactory.NewLogger(F.ToString("outbound/", outboundOptions.Type, "[", tag, "]")),
			tag,
			outboundOptions.Type,
			outboundOptions.Options,
		)
		if err != nil {
			return nil, E.Cause(err, "parse outbound[", i, "]")
		}
		outbounds = append(outbounds, currentOutbound)
	}
	err = router.Initialize(inbounds, outbounds, func() adapter.Outbound {
		defaultOutbound, cErr := direct.NewOutbound(ctx, router, logFactory.NewLogger("outbound/direct"), "direct", option.DirectOutboundOptions{})
		common.Must(cErr)
		outbounds = append(outbounds, defaultOutbound)
		return defaultOutbound
	})
	if err != nil {
		return nil, err
	}
	if platformInterface != nil {
		err = platformInterface.Initialize(ctx, router)
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
		clashServer, err := experimental.NewClashServer(ctx, router, logFactory.(log.ObservableFactory), clashAPIOptions)
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
		inbounds:     inbounds,
		outbounds:    outbounds,
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
		if preService, isPreService := service.(adapter.PreStarter); isPreService {
			monitor.Start("pre-start ", serviceName)
			err := preService.PreStart()
			monitor.Finish()
			if err != nil {
				return E.Cause(err, "pre-start ", serviceName)
			}
		}
	}
	for serviceName, service := range s.preServices2 {
		if preService, isPreService := service.(adapter.PreStarter); isPreService {
			monitor.Start("pre-start ", serviceName)
			err := preService.PreStart()
			monitor.Finish()
			if err != nil {
				return E.Cause(err, "pre-start ", serviceName)
			}
		}
	}
	err = s.router.PreStart()
	if err != nil {
		return E.Cause(err, "pre-start router")
	}
	err = s.startOutbounds()
	if err != nil {
		return err
	}
	return s.router.Start()
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
	for i, in := range s.inbounds {
		var tag string
		if in.Tag() == "" {
			tag = F.ToString(i)
		} else {
			tag = in.Tag()
		}
		err = in.Start()
		if err != nil {
			return E.Cause(err, "initialize inbound/", in.Type(), "[", tag, "]")
		}
	}
	err = s.postStart()
	if err != nil {
		return err
	}
	return s.router.Cleanup()
}

func (s *Box) postStart() error {
	for serviceName, service := range s.postServices {
		err := service.Start()
		if err != nil {
			return E.Cause(err, "start ", serviceName)
		}
	}
	// TODO: reorganize ALL start order
	for _, out := range s.outbounds {
		if lateOutbound, isLateOutbound := out.(adapter.PostStarter); isLateOutbound {
			err := lateOutbound.PostStart()
			if err != nil {
				return E.Cause(err, "post-start outbound/", out.Tag())
			}
		}
	}
	err := s.router.PostStart()
	if err != nil {
		return err
	}
	for _, in := range s.inbounds {
		if lateInbound, isLateInbound := in.(adapter.PostStarter); isLateInbound {
			err = lateInbound.PostStart()
			if err != nil {
				return E.Cause(err, "post-start inbound/", in.Tag())
			}
		}
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
	for i, in := range s.inbounds {
		monitor.Start("close inbound/", in.Type(), "[", i, "]")
		errors = E.Append(errors, in.Close(), func(err error) error {
			return E.Cause(err, "close inbound/", in.Type(), "[", i, "]")
		})
		monitor.Finish()
	}
	for i, out := range s.outbounds {
		monitor.Start("close outbound/", out.Type(), "[", i, "]")
		errors = E.Append(errors, common.Close(out), func(err error) error {
			return E.Cause(err, "close outbound/", out.Type(), "[", i, "]")
		})
		monitor.Finish()
	}
	monitor.Start("close router")
	if err := common.Close(s.router); err != nil {
		errors = E.Append(errors, err, func(err error) error {
			return E.Cause(err, "close router")
		})
	}
	monitor.Finish()
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

func (s *Box) Router() adapter.Router {
	return s.router
}
