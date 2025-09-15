package dialer

import (
	"github.com/sagernet/sing/common/control"
)

type WireGuardListener interface {
	WireGuardControl() control.Func
}
