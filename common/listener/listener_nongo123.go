//go:build !go1.23

package listener

import (
	"net"
	"time"

	"github.com/sagernet/sing/common/control"
)

func setKeepAliveConfig(listener *net.ListenConfig, idle time.Duration, interval time.Duration) {
	listener.KeepAlive = idle
	listener.Control = control.Append(listener.Control, control.SetKeepAlivePeriod(idle, interval))
}
