//go:build !with_clash_api

package include

import (
	"github.com/jobberrt/sing-box/adapter"
	"github.com/jobberrt/sing-box/experimental"
	"github.com/jobberrt/sing-box/log"
	"github.com/jobberrt/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func init() {
	experimental.RegisterClashServerConstructor(func(router adapter.Router, logFactory log.ObservableFactory, options option.ClashAPIOptions) (adapter.ClashServer, error) {
		return nil, E.New(`clash api is not included in this build, rebuild with -tags with_clash_api`)
	})
}
