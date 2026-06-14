//go:build with_usbip && (linux || (darwin && cgo) || windows)

package usbip

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	boxService "github.com/sagernet/sing-box/adapter/service"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-usbip"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
)

type ClientService struct {
	boxService.Adapter
	ctx    context.Context
	logger log.ContextLogger
	inner  *usbip.ClientService
}

func NewClientService(ctx context.Context, logger log.ContextLogger, tag string, options option.USBIPClientServiceOptions) (adapter.Service, error) {
	serviceDialer, err := dialer.NewWithOptions(dialer.Options{
		Context: ctx,
		Options: option.DialerOptions{
			Detour: options.Detour,
		},
		RemoteIsDomain: true,
	})
	if err != nil {
		return nil, E.Cause(err, "create dialer")
	}
	inner, err := usbip.NewClientService(ctx, usbip.ClientOptions{
		Logger:        logger,
		Dialer:        serviceDialer,
		ServerAddress: M.ParseSocksaddr(options.Server),
		Devices:       toDeviceMatches(options.Devices),
	})
	if err != nil {
		return nil, err
	}
	return &ClientService{
		Adapter: boxService.NewAdapter(C.TypeUSBIPClient, tag),
		ctx:     ctx,
		logger:  logger,
		inner:   inner,
	}, nil
}

func (s *ClientService) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	return s.inner.Start()
}

func (s *ClientService) Close() error {
	return s.inner.Close()
}
