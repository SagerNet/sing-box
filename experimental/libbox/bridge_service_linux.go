//go:build linux

package libbox

import (
	"net/netip"

	"github.com/sagernet/sing-box/protocol/bridge"
	E "github.com/sagernet/sing/common/exceptions"
)

func NewBridgeService(options *BridgeOptions) (BridgeSession, error) {
	if options == nil {
		return nil, E.New("missing bridge options")
	}
	serviceOptions := bridge.ServiceOptions{
		BridgeName: options.BridgeName,
		MTU:        int(options.MTU),
		RuleIndex:  int(options.RuleIndex),
		RouteTable: int(options.RouteTable),
	}
	if options.Inet4Port != "" {
		inet4Port, err := netip.ParseAddr(options.Inet4Port)
		if err != nil {
			return nil, E.Cause(err, "parse inet4 port address")
		}
		serviceOptions.Inet4Port = inet4Port
	}
	if options.Inet6Port != "" {
		inet6Port, err := netip.ParseAddr(options.Inet6Port)
		if err != nil {
			return nil, E.Cause(err, "parse inet6 port address")
		}
		serviceOptions.Inet6Port = inet6Port
	}
	service, err := bridge.NewService(serviceOptions)
	if err != nil {
		return nil, err
	}
	return &bridgeServiceSession{service}, nil
}

type bridgeServiceSession struct {
	service *bridge.Service
}

func (s *bridgeServiceSession) FileDescriptor() int32 {
	return int32(s.service.FileDescriptor())
}

func (s *bridgeServiceSession) Name() string {
	return s.service.Name()
}

func (s *bridgeServiceSession) Inet6Active() bool {
	return s.service.Inet6Active()
}

func (s *bridgeServiceSession) SetEgress(interfaceName string) error {
	return s.service.SetEgress(interfaceName)
}

func (s *bridgeServiceSession) Close() error {
	return s.service.Close()
}
