package process

import (
	"context"
	"net/netip"
	"os/user"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-tun"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
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
	ProcessID   uint32
	ProcessPath string
	PackageName string
	User        string
	UserId      int32
}

func FindProcessInfo(searcher Searcher, ctx context.Context, network string, source netip.AddrPort, destination netip.AddrPort) (*Info, error) {
	info, err := searcher.FindProcessInfo(ctx, network, source, destination)
	if err != nil {
		return nil, err
	}
	if info.UserId != -1 {
		osUser, _ := user.LookupId(F.ToString(info.UserId))
		if osUser != nil {
			info.User = osUser.Username
		}
	}
	return info, nil
}
