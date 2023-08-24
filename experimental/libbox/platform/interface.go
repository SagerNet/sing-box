package platform

import (
	"context"
	"io"
	"net/netip"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/process"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/control"
	"github.com/sagernet/sing/common/logger"
)

type Interface interface {
	Initialize(ctx context.Context, router adapter.Router) error
	UsePlatformAutoDetectInterfaceControl() bool
	AutoDetectInterfaceControl() control.Func
	OpenTun(options *tun.Options, platformOptions option.TunPlatformOptions) (tun.Tun, error)
	UsePlatformDefaultInterfaceMonitor() bool
	CreateDefaultInterfaceMonitor(logger logger.Logger) tun.DefaultInterfaceMonitor
	UsePlatformInterfaceGetter() bool
	Interfaces() ([]NetworkInterface, error)
	UnderNetworkExtension() bool
	ClearDNSCache()
	process.Searcher
	io.Writer
}

type NetworkInterface struct {
	Index     int
	MTU       int
	Name      string
	Addresses []netip.Prefix
}
