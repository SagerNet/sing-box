package dialer

import (
	"github.com/sagernet/sing/common/control"
)

type UDPListener interface {
	UDPListenerControl() (control.Func, bool)
}
