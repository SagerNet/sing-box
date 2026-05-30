package dialer

import (
	"net/netip"

	"github.com/sagernet/sing/common/control"
)

type WireGuardListener interface {
	WireGuardControl() control.Func
}

// WireGuardListenerWithBind 支持绑定到指定 IP（同网卡多 IP 场景）
type WireGuardListenerWithBind interface {
	WireGuardListener
	WireGuardBindAddress4() (netip.Addr, bool)
	WireGuardBindAddress6() (netip.Addr, bool)
}
