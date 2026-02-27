//go:build !go1.23

package dialer

import (
	"net"
	"time"

	"github.com/sagernet/sing/common/control"
)

func setKeepAliveConfig(dialer *net.Dialer, idle time.Duration, interval time.Duration) {
	dialer.KeepAlive = idle
	dialer.Control = control.Append(dialer.Control, control.SetKeepAlivePeriod(idle, interval))
}
