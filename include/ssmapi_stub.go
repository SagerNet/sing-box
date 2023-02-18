//go:build !with_ssm_api

package include

import (
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/experimental"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func init() {
	experimental.RegisterSSMServerConstructor(func(router adapter.Router, logger log.Logger, options option.SSMAPIOptions) (adapter.SSMServer, error) {
		return nil, E.New(`SSM api is not included in this build, rebuild with -tags with_ssm_api`)
	})
}
