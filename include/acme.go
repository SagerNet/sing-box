//go:build with_acme

package include

import (
	"github.com/sagernet/sing-box/adapter/service"
	"github.com/sagernet/sing-box/service/acme"
)

func registerACMEService(registry *service.Registry) {
	acme.RegisterService(registry)
}
