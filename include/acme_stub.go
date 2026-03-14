//go:build !with_acme

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

func registerACMEService(registry *service.Registry) {
	service.Register[option.ACMEServiceOptions](registry, C.TypeACME, func(ctx context.Context, logger log.ContextLogger, tag string, options option.ACMEServiceOptions) (adapter.Service, error) {
		return nil, E.New(`ACME is not included in this build, rebuild with -tags with_acme`)
	})
}
