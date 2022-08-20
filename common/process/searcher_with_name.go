//go:build linux && !android

package process

import (
	"context"
	"net/netip"
	"os/user"

	F "github.com/sagernet/sing/common/format"
)

func findProcessInfo(searcher Searcher, ctx context.Context, network string, source netip.AddrPort, destination netip.AddrPort) (*Info, error) {
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
