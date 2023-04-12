//go:build !with_subscribe

package subscribe

import (
	"context"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func ParsePeer(ctx context.Context, tag string, options option.SubscribeOutboundOptions) ([]option.Outbound, error) {
	return nil, E.New(`Subscribe is not included in this build, rebuild with -tags with_subscribe`)
}
