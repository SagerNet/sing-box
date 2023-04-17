package platform

import (
	"io"

	"github.com/sagernet/sing-box/common/process"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/control"
)

type Interface interface {
	AutoDetectInterfaceControl() control.Func
	OpenTun(options *tun.Options, platformOptions option.TunPlatformOptions) (tun.Tun, error)
	process.Searcher
	io.Writer
}
