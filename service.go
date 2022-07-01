package box

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/adapter/route"
	"github.com/sagernet/sing-box/config"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sirupsen/logrus"
)

var _ adapter.Service = (*Service)(nil)

type Service struct {
	logger    *logrus.Logger
	inbounds  []adapter.Inbound
	outbounds []adapter.Outbound
	router    *route.Router
}

func NewService(ctx context.Context, options *config.Config) (service *Service, err error) {
	logger := logrus.New()
	logger.SetLevel(logrus.TraceLevel)
	logger.Formatter.(*logrus.TextFormatter).ForceColors = true
	logger.AddHook(new(log.Hook))
	if options.Log != nil {
		if options.Log.Level != "" {
			logger.Level, err = logrus.ParseLevel(options.Log.Level)
			if err != nil {
				return
			}
		}
	}
	service = &Service{
		logger: logger,
		router: route.NewRouter(logrus.NewEntry(logger).WithFields(logrus.Fields{"prefix": "router: "})),
	}
	if len(options.Inbounds) > 0 {
		for i, inboundOptions := range options.Inbounds {
			var prefix string
			if inboundOptions.Tag != "" {
				prefix = inboundOptions.Tag
			} else {
				prefix = F.ToString(i)
			}
			prefix = F.ToString("inbound/", inboundOptions.Type, "[", prefix, "]: ")
			inboundLogger := logrus.NewEntry(logger).WithFields(logrus.Fields{"prefix": prefix})
			var inboundService adapter.Inbound
			switch inboundOptions.Type {
			case C.TypeDirect:
				inboundService = inbound.NewDirect(ctx, service.router, inboundLogger, inboundOptions.Tag, inboundOptions.DirectOptions)
			case C.TypeSocks:
				inboundService = inbound.NewSocks(ctx, service.router, inboundLogger, inboundOptions.Tag, inboundOptions.SocksOptions)
			case C.TypeHTTP:
				inboundService = inbound.NewHTTP(ctx, service.router, inboundLogger, inboundOptions.Tag, inboundOptions.HTTPOptions)
			case C.TypeMixed:
				inboundService = inbound.NewMixed(ctx, service.router, inboundLogger, inboundOptions.Tag, inboundOptions.MixedOptions)
			case C.TypeShadowsocks:
				inboundService, err = inbound.NewShadowsocks(ctx, service.router, inboundLogger, inboundOptions.Tag, inboundOptions.ShadowsocksOptions)
			default:
				err = E.New("unknown inbound type: " + inboundOptions.Type)
			}
			if err != nil {
				return
			}
			service.inbounds = append(service.inbounds, inboundService)
		}
	}
	for i, outboundOptions := range options.Outbounds {
		var prefix string
		if outboundOptions.Tag != "" {
			prefix = outboundOptions.Tag
		} else {
			prefix = F.ToString(i)
		}
		prefix = F.ToString("outbound/", outboundOptions.Type, "[", prefix, "]: ")
		outboundLogger := logrus.NewEntry(logger).WithFields(logrus.Fields{"prefix": prefix})
		var outboundHandler adapter.Outbound
		switch outboundOptions.Type {
		case C.TypeDirect:
			outboundHandler = outbound.NewDirect(service.router, outboundLogger, outboundOptions.Tag, outboundOptions.DirectOptions)
		case C.TypeShadowsocks:
			outboundHandler, err = outbound.NewShadowsocks(service.router, outboundLogger, outboundOptions.Tag, outboundOptions.ShadowsocksOptions)
		default:
			err = E.New("unknown outbound type: " + outboundOptions.Type)
		}
		if err != nil {
			return
		}
		service.outbounds = append(service.outbounds, outboundHandler)
		service.router.AddOutbound(outboundHandler)
	}
	if len(service.outbounds) == 0 {
		service.outbounds = append(service.outbounds, outbound.NewDirect(nil, logger, "direct", &config.DirectOutboundOptions{}))
		service.router.AddOutbound(service.outbounds[0])
	}
	return
}

func (s *Service) Start() error {
	for _, inbound := range s.inbounds {
		err := inbound.Start()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) Close() error {
	for _, inbound := range s.inbounds {
		inbound.Close()
	}
	for _, outbound := range s.outbounds {
		common.Close(outbound)
	}
	return nil
}
