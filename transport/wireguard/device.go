package wireguard

import (
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/wireguard-go/tun"
)

type Device interface {
	tun.Device
	N.Dialer
	Start() error
	// NewEndpoint() (stack.LinkEndpoint, error)
}
