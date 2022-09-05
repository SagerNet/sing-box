package wireguard

import (
	N "github.com/sagernet/sing/common/network"

	"golang.zx2c4.com/wireguard/tun"
)

type Device interface {
	tun.Device
	N.Dialer
	Start() error
	// NewEndpoint() (stack.LinkEndpoint, error)
}
