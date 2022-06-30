package box

import (
	"context"
	"net/netip"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/direct"
	"github.com/sagernet/sing-box/adapter/http"
	"github.com/sagernet/sing-box/adapter/mixed"
	"github.com/sagernet/sing-box/adapter/shadowsocks"
	"github.com/sagernet/sing-box/adapter/socks"
	"github.com/sagernet/sing-box/config"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/route"
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
			var inbound adapter.InboundHandler

			var listenOptions config.ListenOptions
			switch inboundOptions.Type {
			case C.TypeDirect:
				listenOptions = inboundOptions.DirectOptions.ListenOptions
				inbound = direct.NewInbound(service.router, inboundLogger, inboundOptions.DirectOptions)
			case C.TypeSocks:
				listenOptions = inboundOptions.SocksOptions.ListenOptions
				inbound = socks.NewInbound(service.router, inboundLogger, inboundOptions.SocksOptions)
			case C.TypeHTTP:
				listenOptions = inboundOptions.HTTPOptions.ListenOptions
				inbound = http.NewInbound(service.router, inboundLogger, inboundOptions.HTTPOptions)
			case C.TypeMixed:
				listenOptions = inboundOptions.MixedOptions.ListenOptions
				inbound = mixed.NewInbound(service.router, inboundLogger, inboundOptions.MixedOptions)
			case C.TypeShadowsocks:
				listenOptions = inboundOptions.ShadowsocksOptions.ListenOptions
				inbound, err = shadowsocks.NewInbound(service.router, inboundLogger, inboundOptions.ShadowsocksOptions)
			default:
				err = E.New("unknown inbound type: " + inboundOptions.Type)
			}
			if err != nil {
				return
			}
			service.inbounds = append(service.inbounds, adapter.NewDefaultInboundService(
				ctx,
				inboundOptions.Tag,
				inboundLogger,
				netip.AddrPortFrom(netip.Addr(listenOptions.Listen), listenOptions.Port),
				listenOptions.TCPFastOpen,
				inbound,
			))
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
		var outbound adapter.Outbound
		switch outboundOptions.Type {
		case C.TypeDirect:
			outbound = direct.NewOutbound(outboundOptions.Tag, service.router, outboundLogger, outboundOptions.DirectOptions)
		case C.TypeShadowsocks:
			outbound, err = shadowsocks.NewOutbound(outboundOptions.Tag, service.router, outboundLogger, outboundOptions.ShadowsocksOptions)
		default:
			err = E.New("unknown outbound type: " + outboundOptions.Type)
		}
		if err != nil {
			return
		}
		service.outbounds = append(service.outbounds, outbound)
		service.router.AddOutbound(outbound)
	}
	if len(service.outbounds) == 0 {
		service.outbounds = append(service.outbounds, direct.NewOutbound("direct", service.router, logger, &config.DirectOutboundOptions{}))
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
