package process

import (
	"context"
	"net/netip"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-tun"
	E "github.com/sagernet/sing/common/exceptions"
)

type Searcher interface {
	FindProcessInfo(ctx context.Context, network string, source netip.AddrPort, destination netip.AddrPort) (*Info, error)
}

var ErrNotFound = E.New("process not found")

type Config struct {
	Logger         log.ContextLogger
	PackageManager tun.PackageManager
}

type Info struct {
	ProcessPath string
	PackageName string
	User        string
	UserId      int32
}

func FindProcessInfo(searcher Searcher, ctx context.Context, network string, source netip.AddrPort, destination netip.AddrPort) (*Info, error) {
	info, err := findProcessInfo(searcher, ctx, network, source, destination)
	if err != nil {
		if source.Addr().Is4In6() {
			info, err = findProcessInfo(searcher, ctx, network, netip.AddrPortFrom(netip.AddrFrom4(source.Addr().As4()), source.Port()), destination)
		}
	}
	return info, err
}
