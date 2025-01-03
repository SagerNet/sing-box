//go:build !windows

package include

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func registerNDISInbound(registry *inbound.Registry) {
	inbound.Register[option.NDISInboundOptions](registry, C.TypeNDIS, func(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.NDISInboundOptions) (adapter.Inbound, error) {
		return nil, E.New("NDIS is only supported in windows")
	})
}
