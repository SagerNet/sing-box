//go:build !linux || !with_gvisor

package xdp

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func RegisterInbound(registry *inbound.Registry) {
	inbound.Register[option.XDPInboundOptions](registry, C.TypeXDP, func(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.XDPInboundOptions) (adapter.Inbound, error) {
		return nil, E.New("XDP requires Linux with gVisor support, rebuild with -tags with_gvisor on Linux")
	})
}
