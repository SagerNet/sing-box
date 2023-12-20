//go:build !with_wireguard

package dialer

import (
	"github.com/sagernet/sing/common/control"
)

var wgControlFns []control.Func
