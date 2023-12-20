//go:build with_wireguard

package dialer

import (
	"github.com/sagernet/wireguard-go/conn"
)

var _ WireGuardListener = (conn.Listener)(nil)

var wgControlFns = conn.ControlFns
