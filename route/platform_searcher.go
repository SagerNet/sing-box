package route

import (
	"context"
	"net/netip"
	"syscall"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/process"
	N "github.com/sagernet/sing/common/network"
)

type platformSearcher struct {
	platform adapter.PlatformInterface
}

func newPlatformSearcher(platform adapter.PlatformInterface) process.Searcher {
	return &platformSearcher{platform: platform}
}

func (s *platformSearcher) FindProcessInfo(ctx context.Context, network string, source netip.AddrPort, destination netip.AddrPort) (*adapter.ConnectionOwner, error) {
	if !s.platform.UsePlatformConnectionOwnerFinder() {
		return nil, process.ErrNotFound
	}

	var ipProtocol int32
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		ipProtocol = syscall.IPPROTO_TCP
	case N.NetworkUDP:
		ipProtocol = syscall.IPPROTO_UDP
	default:
		return nil, process.ErrNotFound
	}

	request := &adapter.FindConnectionOwnerRequest{
		IpProtocol:         ipProtocol,
		SourceAddress:      source.Addr().String(),
		SourcePort:         int32(source.Port()),
		DestinationAddress: destination.Addr().String(),
		DestinationPort:    int32(destination.Port()),
	}

	return s.platform.FindConnectionOwner(request)
}
