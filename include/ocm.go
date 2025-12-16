//go:build with_ocm

package include

import (
	"github.com/sagernet/sing-box/adapter/service"
	"github.com/sagernet/sing-box/service/ocm"
)

func registerOCMService(registry *service.Registry) {
	ocm.RegisterService(registry)
}
