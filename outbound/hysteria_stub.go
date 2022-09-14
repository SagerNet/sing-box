//go:build !with_quic

package outbound

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	I "github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

func NewHysteria(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.HysteriaOutboundOptions) (adapter.Outbound, error) {
	return nil, I.ErrQUICNotIncluded
}
