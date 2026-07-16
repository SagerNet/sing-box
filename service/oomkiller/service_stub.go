//go:build !darwin || !cgo

package oomkiller

import (
	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
)

func (s *Service) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	if !s.timerConfig.policyMode.hasTimerMode() {
		return E.New("memory pressure monitoring is not available on this platform without memory_limit")
	}
	s.adaptiveTimer = newAdaptiveTimer(s.logger, s.network, s.timerConfig, s.writeOOMReport)
	s.adaptiveTimer.start()
	return nil
}

func (s *Service) Close() error {
	if s.adaptiveTimer != nil {
		s.adaptiveTimer.stop()
	}
	return nil
}
