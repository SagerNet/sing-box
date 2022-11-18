//go:build !with_bgp

package bgp

import (
	"context"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func newBgpServer(ctx context.Context, op option.BgpOptions, logger log.ContextLogger) (BgpAPI, error) {
	return nil, E.New(`go-bgp is not included in this build, rebuild with -tags with_bgp`)
}
