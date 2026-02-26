//go:build !darwin || !cgo

package oomkiller

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	boxService "github.com/sagernet/sing-box/adapter/service"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func RegisterService(registry *boxService.Registry) {
	boxService.Register[option.OOMKillerServiceOptions](registry, C.TypeOOMKiller, NewService)
}

type Service struct {
	boxService.Adapter
}

func NewService(ctx context.Context, logger log.ContextLogger, tag string, options option.OOMKillerServiceOptions) (adapter.Service, error) {
	return &Service{
		Adapter: boxService.NewAdapter(C.TypeOOMKiller, tag),
	}, nil
}

func (s *Service) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	return E.New("memory pressure monitoring is not available on this platform")
}

func (s *Service) Close() error {
	return nil
}
