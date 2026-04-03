//go:build !darwin || !cgo

package oomkiller

import (
	"context"
	"sync/atomic"

	"github.com/sagernet/sing-box/adapter"
	boxService "github.com/sagernet/sing-box/adapter/service"
	boxConstant "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/byteformats"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/memory"
	"github.com/sagernet/sing/service"
)

func RegisterService(registry *boxService.Registry) {
	boxService.Register[option.OOMKillerServiceOptions](registry, boxConstant.TypeOOMKiller, NewService)
}

type Service struct {
	boxService.Adapter
	ctx            context.Context
	logger         log.ContextLogger
	router         adapter.Router
	memoryLimit    uint64
	hasTimerMode   bool
	useAvailable   bool
	killerDisabled bool
	timerConfig    timerConfig
	adaptiveTimer  *adaptiveTimer
	lastReportTime atomic.Int64
}

func NewService(ctx context.Context, logger log.ContextLogger, tag string, options option.OOMKillerServiceOptions) (adapter.Service, error) {
	s := &Service{
		Adapter:        boxService.NewAdapter(boxConstant.TypeOOMKiller, tag),
		ctx:            ctx,
		logger:         logger,
		router:         service.FromContext[adapter.Router](ctx),
		killerDisabled: options.KillerDisabled,
	}

	if options.MemoryLimitOverride > 0 {
		s.memoryLimit = options.MemoryLimitOverride
		s.hasTimerMode = true
	} else if options.MemoryLimit != nil {
		s.memoryLimit = options.MemoryLimit.Value()
		if s.memoryLimit > 0 {
			s.hasTimerMode = true
		}
	}
	if !s.hasTimerMode && memory.AvailableSupported() {
		s.useAvailable = true
		s.hasTimerMode = true
	}

	config, err := buildTimerConfig(options, s.memoryLimit, s.useAvailable, s.killerDisabled)
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
	s.adaptiveTimer = newAdaptiveTimer(s.logger, s.router, s.timerConfig,
		func(usage uint64) { s.writeOOMReport(usage) },
		nil,
	)
	s.adaptiveTimer.start(0)
	if s.useAvailable {
		s.logger.Info("started memory monitor with available memory detection")
	} else {
		s.logger.Info("started memory monitor with limit: ", byteformats.FormatMemoryBytes(s.memoryLimit))
	}
	return nil
}

func (s *Service) Close() error {
	if s.adaptiveTimer != nil {
		s.adaptiveTimer.stop()
	}
	return nil
}
