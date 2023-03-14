//go:build !with_quic

package inbound

import (
	"context"

	"github.com/jobberrt/sing-box/adapter"
	C "github.com/jobberrt/sing-box/constant"
	"github.com/jobberrt/sing-box/log"
	"github.com/jobberrt/sing-box/option"
)

func NewHysteria(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.HysteriaInboundOptions) (adapter.Inbound, error) {
	return nil, C.ErrQUICNotIncluded
}
