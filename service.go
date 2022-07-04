package box

import (
	"context"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/inbound"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/outbound"
	"github.com/sagernet/sing-box/route"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
)

var _ adapter.Service = (*Service)(nil)

type Service struct {
	router    adapter.Router
	logger    log.Logger
	inbounds  []adapter.Inbound
	outbounds []adapter.Outbound
	createdAt time.Time
}

func NewService(ctx context.Context, options option.Options) (*Service, error) {
	createdAt := time.Now()
	logger, err := log.NewLogger(common.PtrValueOrDefault(options.Log))
	if err != nil {
		return nil, E.Cause(err, "parse log options")
	}
	router, err := route.NewRouter(ctx, logger, common.PtrValueOrDefault(options.Route))
	if err != nil {
		return nil, E.Cause(err, "parse route options")
	}
	inbounds := make([]adapter.Inbound, 0, len(options.Inbounds))
	outbounds := make([]adapter.Outbound, 0, len(options.Outbounds))
	for i, inboundOptions := range options.Inbounds {
		var in adapter.Inbound
		in, err = inbound.New(ctx, router, logger, i, inboundOptions)
		if err != nil {
			return nil, E.Cause(err, "parse inbound[", i, "]")
		}
		inbounds = append(inbounds, in)
	}
	for i, outboundOptions := range options.Outbounds {
		var out adapter.Outbound
		out, err = outbound.New(router, logger, i, outboundOptions)
		if err != nil {
			return nil, E.Cause(err, "parse outbound[", i, "]")
		}
		outbounds = append(outbounds, out)
	}
	err = router.Initialize(outbounds, func() adapter.Outbound {
		out, oErr := outbound.New(router, logger, 0, option.Outbound{Type: "direct", Tag: "default"})
		common.Must(oErr)
		outbounds = append(outbounds, out)
		return out
	})
	if err != nil {
		return nil, err
	}
	return &Service{
		router:    router,
		logger:    logger,
		inbounds:  inbounds,
		outbounds: outbounds,
		createdAt: createdAt,
	}, nil
}

func (s *Service) Start() error {
	err := s.logger.Start()
	if err != nil {
		return err
	}
	err = s.router.Start()
	if err != nil {
		return err
	}
	for _, in := range s.inbounds {
		err = in.Start()
		if err != nil {
			return err
		}
	}
	s.logger.Info("sing-box started (", F.Seconds(time.Since(s.createdAt).Seconds()), "s)")
	return nil
}

func (s *Service) Close() error {
	for _, in := range s.inbounds {
		in.Close()
	}
	for _, out := range s.outbounds {
		common.Close(out)
	}
	return common.Close(
		s.router,
		s.logger,
	)
}
