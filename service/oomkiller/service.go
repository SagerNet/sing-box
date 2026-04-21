package oomkiller

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	boxService "github.com/sagernet/sing-box/adapter/service"
	boxConstant "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/service"
)

type OOMReporter interface {
	WriteReport(memoryUsage uint64) error
	WriteDraft(memoryUsage uint64) error
	DiscardDraft() error
}

func RegisterService(registry *boxService.Registry) {
	boxService.Register[option.OOMKillerServiceOptions](registry, boxConstant.TypeOOMKiller, NewService)
}

type Service struct {
	boxService.Adapter
	ctx            context.Context
	logger         log.ContextLogger
	router         adapter.Router
	timerConfig    timerConfig
	adaptiveTimer  *adaptiveTimer
	lastReportTime atomic.Int64
	draftCancelled atomic.Bool
}

func NewService(ctx context.Context, logger log.ContextLogger, tag string, options option.OOMKillerServiceOptions) (adapter.Service, error) {
	memoryLimit, mode := resolvePolicyMode(ctx, options)
	config, err := buildTimerConfig(options, memoryLimit, mode, options.KillerDisabled)
	if err != nil {
		return nil, err
	}
	return &Service{
		Adapter:     boxService.NewAdapter(boxConstant.TypeOOMKiller, tag),
		ctx:         ctx,
		logger:      logger,
		router:      service.FromContext[adapter.Router](ctx),
		timerConfig: config,
	}, nil
}

func (s *Service) createTimer() {
	s.adaptiveTimer = newAdaptiveTimer(s.logger, s.router, s.timerConfig, s.writeOOMReport)
}

func (s *Service) startTimer() {
	s.createTimer()
	s.adaptiveTimer.start()
}

func (s *Service) stopTimer() {
	if s.adaptiveTimer != nil {
		s.adaptiveTimer.stop()
	}
}

func (s *Service) writeOOMReport(memoryUsage uint64) {
	now := time.Now().Unix()
	lastReport := s.lastReportTime.Load()
	if now-lastReport < 3600 {
		return
	}
	if !s.lastReportTime.CompareAndSwap(lastReport, now) {
		return
	}
	reporter := service.FromContext[OOMReporter](s.ctx)
	if reporter == nil {
		return
	}
	err := reporter.WriteReport(memoryUsage)
	if err != nil {
		s.logger.Warn("failed to write OOM report: ", err)
	} else {
		s.logger.Info("OOM report saved")
	}
}

func (s *Service) writeOOMDraft(memoryUsage uint64) {
	if s.draftCancelled.Load() {
		return
	}
	reporter := service.FromContext[OOMReporter](s.ctx)
	if reporter == nil {
		return
	}
	err := reporter.WriteDraft(memoryUsage)
	if s.draftCancelled.Load() {
		reporter.DiscardDraft()
		return
	}
	if err != nil {
		s.logger.Warn("failed to write OOM draft: ", err)
	} else {
		s.logger.Warn("OOM draft saved")
	}
}

func (s *Service) discardOOMDraft() {
	s.draftCancelled.Store(true)
	reporter := service.FromContext[OOMReporter](s.ctx)
	if reporter == nil {
		return
	}
	err := reporter.DiscardDraft()
	if err != nil {
		s.logger.Warn("failed to discard OOM draft: ", err)
	} else {
		s.logger.Info("OOM draft discarded")
	}
}
