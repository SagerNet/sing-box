//go:build go1.23

package listener

import (
	"net"
	"time"
)

func setKeepAliveConfig(listener *net.ListenConfig, idle time.Duration, interval time.Duration) {
	listener.KeepAliveConfig = net.KeepAliveConfig{
		Enable:   true,
		Idle:     idle,
		Interval: interval,
	}
}
