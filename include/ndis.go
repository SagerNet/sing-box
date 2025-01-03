//go:build windows && with_gvisor

package include

import (
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/protocol/ndis"
)

func registerNDISInbound(registry *inbound.Registry) {
	ndis.RegisterInbound(registry)
}
