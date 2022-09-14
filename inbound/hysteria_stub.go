//go:build !with_quic

package inbound

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	I "github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

func NewHysteria(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.HysteriaInboundOptions) (adapter.Inbound, error) {
	return nil, I.ErrQUICNotIncluded
}
