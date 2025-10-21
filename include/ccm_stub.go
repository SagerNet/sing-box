//go:build !with_ccm

package include

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/service"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func registerCCMService(registry *service.Registry) {
	service.Register[option.CCMServiceOptions](registry, C.TypeCCM, func(ctx context.Context, logger log.ContextLogger, tag string, options option.CCMServiceOptions) (adapter.Service, error) {
		return nil, E.New(`CCM is not included in this build, rebuild with -tags with_CCM`)
	})
}
