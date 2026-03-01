//go:build !darwin || !cgo

package oomkiller

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	boxService "github.com/sagernet/sing-box/adapter/service"
	boxConstant "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/memory"
	"github.com/sagernet/sing/service"
)

func RegisterService(registry *boxService.Registry) {
	boxService.Register[option.OOMKillerServiceOptions](registry, boxConstant.TypeOOMKiller, NewService)
}

type Service struct {
	boxService.Adapter
	logger        log.ContextLogger
	router        adapter.Router
	adaptiveTimer *adaptiveTimer
	timerConfig   timerConfig
	hasTimerMode  bool
	useAvailable  bool
	memoryLimit   uint64
}

func NewService(ctx context.Context, logger log.ContextLogger, tag string, options option.OOMKillerServiceOptions) (adapter.Service, error) {
	s := &Service{
		Adapter: boxService.NewAdapter(boxConstant.TypeOOMKiller, tag),
		logger:  logger,
		router:  service.FromContext[adapter.Router](ctx),
	}

	if options.MemoryLimit != nil {
		s.memoryLimit = options.MemoryLimit.Value()
	}
	if s.memoryLimit > 0 {
		s.hasTimerMode = true
	} else if memory.AvailableSupported() {
		s.useAvailable = true
		s.hasTimerMode = true
	}

	config, err := buildTimerConfig(options, s.memoryLimit, s.useAvailable)
	if err != nil {
		return nil, err
	}
	s.timerConfig = config

	return s, nil
}

func (s *Service) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	if !s.hasTimerMode {
		return E.New("memory pressure monitoring is not available on this platform without memory_limit")
	}
	s.adaptiveTimer = newAdaptiveTimer(s.logger, s.router, s.timerConfig)
	s.adaptiveTimer.start(0)
	if s.useAvailable {
		s.logger.Info("started memory monitor with available memory detection")
	} else {
		s.logger.Info("started memory monitor with limit: ", s.memoryLimit/(1024*1024), " MiB")
	}
	return nil
}

func (s *Service) Close() error {
	if s.adaptiveTimer != nil {
		s.adaptiveTimer.stop()
	}
	return nil
}
