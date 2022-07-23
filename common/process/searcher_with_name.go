//go:build cgo && linux && !android

package process

import (
	"context"
	"net/netip"
	"os/user"

	F "github.com/sagernet/sing/common/format"
)

func FindProcessInfo(searcher Searcher, ctx context.Context, network string, srcIP netip.Addr, srcPort int) (*Info, error) {
	info, err := searcher.FindProcessInfo(ctx, network, srcIP, srcPort)
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
