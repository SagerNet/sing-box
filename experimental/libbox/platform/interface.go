package platform

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/process"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/control"
	"github.com/sagernet/sing/common/logger"
)

const (
	PerAppProxyModeDisabled int32 = iota
	PerAppProxyModeExclude
	PerAppProxyModeInclude
)

type Interface interface {
	Initialize(ctx context.Context, router adapter.Router) error
	UsePlatformAutoDetectInterfaceControl() bool
	AutoDetectInterfaceControl() control.Func
	OpenTun(options *tun.Options, platformOptions option.TunPlatformOptions) (tun.Tun, error)
	UsePlatformDefaultInterfaceMonitor() bool
	CreateDefaultInterfaceMonitor(logger logger.Logger) tun.DefaultInterfaceMonitor
	UsePlatformInterfaceGetter() bool
	Interfaces() ([]control.Interface, error)
	UnderNetworkExtension() bool
	IncludeAllNetworks() bool
	ClearDNSCache()
	ReadWIFIState() adapter.WIFIState
	PerAppProxyList() ([]uint32, error)
	PerAppProxyMode() int32
	process.Searcher
}
