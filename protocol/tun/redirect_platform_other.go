//go:build !linux

package tun

import (
	"github.com/sagernet/sing-tun"
	E "github.com/sagernet/sing/common/exceptions"
)

func newPlatformAutoRedirect(inbound *Inbound) (tun.AutoRedirect, error) {
	return nil, E.New("platform auto-redirect is only supported on Linux")
}
