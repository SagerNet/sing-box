package oomkiller

import (
	"context"
	"sync/atomic"

	"github.com/sagernet/sing-box/adapter"
	boxService "github.com/sagernet/sing-box/adapter/service"
	boxConstant "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/service"
)

func RegisterService(registry *boxService.Registry) {
	boxService.Register[option.OOMKillerServiceOptions](registry, boxConstant.TypeOOMKiller, NewService)
}

type Service struct {
	boxService.Adapter
	ctx           context.Context
	logger        log.ContextLogger
	network       adapter.NetworkManager
	connections   adapter.ConnectionManager
	recorder      *Recorder
	timerConfig   timerConfig
	adaptiveTimer *adaptiveTimer
	pressure      atomic.Uint32
}

func NewService(ctx context.Context, logger log.ContextLogger, tag string, options option.OOMKillerServiceOptions) (adapter.Service, error) {
	memoryLimit, mode := resolvePolicyMode(ctx, options)
	config, err := buildTimerConfig(options, memoryLimit, mode, options.KillerDisabled)
	if err != nil {
		return nil, err
	}
	s := &Service{
		Adapter:     boxService.NewAdapter(boxConstant.TypeOOMKiller, tag),
		ctx:         ctx,
		logger:      logger,
		network:     service.FromContext[adapter.NetworkManager](ctx),
		connections: service.FromContext[adapter.ConnectionManager](ctx),
		recorder:    service.FromContext[*Recorder](ctx),
		timerConfig: config,
	}
	service.MustRegister[*Service](ctx, s)
	return s, nil
}

func (s *Service) MemoryPressure() tun.MemoryPressure {
	return tun.MemoryPressure(s.pressure.Load())
}

func (s *Service) startTimer() error {
	if !s.timerConfig.policyMode.hasTimerMode() {
		return E.New("memory pressure monitoring is not available on this platform without memory_limit")
	}
	s.adaptiveTimer = newAdaptiveTimer(s.logger, s.network, s.connections, service.FromContext[adapter.CacheFile](s.ctx), s.recorder, &s.pressure, s.timerConfig)
	var carriedState *timerState
	if s.recorder != nil {
		carriedState = s.recorder.instanceStarted(s.timerConfig, s.adaptiveTimer.limitThresholds)
	}
	s.adaptiveTimer.start(carriedState)
	return nil
}

func (s *Service) stopTimer() {
	var state timerState
	if s.adaptiveTimer != nil {
		state = s.adaptiveTimer.stop()
	}
	s.pressure.Store(uint32(tun.MemoryPressureNone))
	if s.recorder != nil {
		s.recorder.instanceStopped(state)
	}
}
