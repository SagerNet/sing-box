//go:build !with_metric_api

package include

import (
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/experimental"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func init() {
	experimental.RegisterMetricServerConstructor(func(logger log.Logger, options option.MetricOptions) (adapter.MetricService, error) {
		return nil, E.New(`metric api is not included in this build, rebuild with -tags with_metric_api`)
	})
}
