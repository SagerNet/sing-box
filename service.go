package box

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/adapter/route"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
)

var _ adapter.Service = (*Service)(nil)

type Service struct {
	router    adapter.Router
	logger    log.Logger
	inbounds  []adapter.Inbound
	outbounds []adapter.Outbound
}

func NewService(ctx context.Context, options option.Options) (*Service, error) {
	logger, err := log.NewLogger(common.PtrValueOrDefault(options.Log))
	if err != nil {
		return nil, err
	}
	router, err := route.NewRouter(ctx, logger, common.PtrValueOrDefault(options.Route))
	if err != nil {
		return nil, err
	}
	inbounds := make([]adapter.Inbound, 0, len(options.Inbounds))
	outbounds := make([]adapter.Outbound, 0, len(options.Outbounds))
	for i, inboundOptions := range options.Inbounds {
		var inboundService adapter.Inbound
		inboundService, err = inbound.New(ctx, router, logger, i, inboundOptions)
		if err != nil {
			return nil, err
		}
		inbounds = append(inbounds, inboundService)
	}
	for i, outboundOptions := range options.Outbounds {
		var outboundService adapter.Outbound
		outboundService, err = outbound.New(router, logger, i, outboundOptions)
		if err != nil {
			return nil, err
		}
		outbounds = append(outbounds, outboundService)
	}
	if len(outbounds) == 0 {
		outbounds = append(outbounds, outbound.NewDirect(nil, logger, "direct", option.DirectOutboundOptions{}))
	}
	router.UpdateOutbounds(outbounds)
	return &Service{
		router:    router,
		logger:    logger,
		inbounds:  inbounds,
		outbounds: outbounds,
	}, nil
}

func (s *Service) Start() error {
	for _, in := range s.inbounds {
		err := in.Start()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) Close() error {
	for _, in := range s.inbounds {
		in.Close()
	}
	for _, out := range s.outbounds {
		common.Close(out)
	}
	s.logger.Close()
	s.router.Close()
	return nil
}
