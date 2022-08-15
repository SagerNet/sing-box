package process

import (
	"context"
	"net/netip"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-tun"
	E "github.com/sagernet/sing/common/exceptions"
)

type Searcher interface {
	FindProcessInfo(ctx context.Context, network string, srcIP netip.Addr, srcPort int) (*Info, error)
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
