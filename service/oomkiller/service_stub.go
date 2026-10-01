//go:build !darwin || !cgo

package oomkiller

import (
	"github.com/sagernet/sing-box/adapter"
)

func (s *Service) Start(stage adapter.StartStage, scope *adapter.Scope) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	err := s.startTimer()
	if err != nil {
		return err
	}
	scope.Add(func() error {
		s.stopTimer()
		return nil
	})
	return nil
}
