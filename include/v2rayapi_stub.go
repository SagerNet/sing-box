//go:build !with_v2ray_api

package include

import (
	"github.com/jobberrt/sing-box/adapter"
	"github.com/jobberrt/sing-box/experimental"
	"github.com/jobberrt/sing-box/log"
	"github.com/jobberrt/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func init() {
	experimental.RegisterV2RayServerConstructor(func(logger log.Logger, options option.V2RayAPIOptions) (adapter.V2RayServer, error) {
		return nil, E.New(`v2ray api is not included in this build, rebuild with -tags with_v2ray_api`)
	})
}
