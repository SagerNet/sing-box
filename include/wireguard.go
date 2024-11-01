//go:build with_wireguard

package include

import (
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/protocol/wireguard"
)

func registerWireGuardOutbound(registry *outbound.Registry) {
	wireguard.RegisterOutbound(registry)
}
