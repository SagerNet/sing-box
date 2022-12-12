//go:build !with_mtproto

package inbound

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func NewMTProto(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.MTProtoInboundOptions) (adapter.Inbound, error) {
	return nil, E.New(`MTProto is not included in this build, rebuild with -tags with_mtproto`)
}
