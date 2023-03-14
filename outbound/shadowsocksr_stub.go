//go:build !with_shadowsocksr

package outbound

import (
	"context"

	"github.com/jobberrt/sing-box/adapter"
	"github.com/jobberrt/sing-box/log"
	"github.com/jobberrt/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func NewShadowsocksR(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.ShadowsocksROutboundOptions) (adapter.Outbound, error) {
	return nil, E.New(`ShadowsocksR is not included in this build, rebuild with -tags with_shadowsocksr`)
}
