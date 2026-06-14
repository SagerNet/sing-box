//go:build with_usbip && (linux || (darwin && cgo) || windows)

package usbip

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	boxService "github.com/sagernet/sing-box/adapter/service"
	"github.com/sagernet/sing-box/common/listener"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-usbip"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
)

type ServerService struct {
	boxService.Adapter
	ctx    context.Context
	logger log.ContextLogger
	inner  *usbip.ServerService
}

type dynamicServerService struct {
	ServerService
	host *usbip.DynamicHost
}

var _ adapter.USBIPDynamicServer = (*dynamicServerService)(nil)

func NewServerService(ctx context.Context, logger log.ContextLogger, tag string, options option.USBIPServerServiceOptions) (adapter.Service, error) {
	listenOptions := options.ListenOptions
	if listenOptions.ListenPort == 0 {
		listenOptions.ListenPort = usbip.DefaultPort
	}
	boxListener := listener.New(listener.Options{
		Context: ctx,
		Logger:  logger,
		Network: []string{N.NetworkTCP},
		Listen:  listenOptions,
	})
	serverOptions := usbip.ServerOptions{
		Logger: logger,
		Listen: func(context.Context) (net.Listener, error) {
			return boxListener.ListenTCP()
		},
	}
	base := ServerService{
		Adapter: boxService.NewAdapter(C.TypeUSBIPServer, tag),
		ctx:     ctx,
		logger:  logger,
	}

	providerType := options.Provider
	if providerType == "" {
		providerType = option.USBIPProviderDefault
	}
	switch providerType {
	case option.USBIPProviderDefault:
		defaultOptions, isDefault := options.Options.(*option.USBIPDefaultProviderOptions)
		if isDefault {
			serverOptions.Devices = toDeviceMatches(defaultOptions.Devices)
		}
		inner, err := usbip.NewServerService(ctx, serverOptions)
		if err != nil {
			return nil, err
		}
		base.inner = inner
		return &base, nil
	case option.USBIPProviderDynamic:
		host := usbip.NewDynamicHost(logger)
		inner, err := usbip.NewDynamicServerService(ctx, serverOptions, host)
		if err != nil {
			return nil, err
		}
		base.inner = inner
		return &dynamicServerService{ServerService: base, host: host}, nil
	default:
		return nil, E.New("unknown usbip provider type: ", providerType)
	}
}

func (s *ServerService) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	return s.inner.Start()
}

func (s *ServerService) Close() error {
	return s.inner.Close()
}

func (s *dynamicServerService) AddDevice(info usbip.DynamicDeviceInfo, transport usbip.DeviceTransport) (string, error) {
	return s.host.AddDevice(info, transport)
}

func (s *dynamicServerService) RemoveDevice(busID string) {
	s.host.RemoveDevice(busID)
}

func (s *dynamicServerService) SubscribeDevices(ctx context.Context, listener func([]usbip.ControlDeviceInfo)) {
	s.inner.SubscribeDevices(ctx, listener)
}
