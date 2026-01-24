package process

import (
	"context"
	"net/netip"
	"os/user"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-tun"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
)

type Searcher interface {
	FindProcessInfo(ctx context.Context, network string, source netip.AddrPort, destination netip.AddrPort) (*adapter.ConnectionOwner, error)
}

var ErrNotFound = E.New("process not found")

type Config struct {
	Logger         log.ContextLogger
	PackageManager tun.PackageManager
}

func FindProcessInfo(searcher Searcher, ctx context.Context, network string, source netip.AddrPort, destination netip.AddrPort) (*adapter.ConnectionOwner, error) {
	info, err := searcher.FindProcessInfo(ctx, network, source, destination)
	if err != nil {
		return nil, err
	}
	if info.UserId != -1 {
		osUser, _ := user.LookupId(F.ToString(info.UserId))
		if osUser != nil {
			info.UserName = osUser.Username
		}
	}
	return info, nil
}
