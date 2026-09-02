//go:build !darwin || !cgo

package oomkiller

import (
	"github.com/sagernet/sing-box/adapter"
)

func (s *Service) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	return s.startTimer()
}

func (s *Service) Close() error {
	s.stopTimer()
	return nil
}
