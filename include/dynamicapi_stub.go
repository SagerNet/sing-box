//go:build !with_dynamic_api

package include

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/experimental"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func init() {
	experimental.RegisterDynamicManagerConstructor(func(ctx context.Context, logger log.ContextLogger, options option.DynamicAPIOptions) (adapter.DynamicManager, error) {
		return nil, E.New(`dynamic api is not included in this build, rebuild with -tags with_dynamic_api`)
	})
}
