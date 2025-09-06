package wsc

import (
	"net"
	"time"

	"github.com/itsabgr/ge"
)

func nowns() int64 {
	return time.Now().UnixNano()
}

func isTimeoutErr(err error) bool {
	if nErr, ok := ge.As[net.Error](err); ok && nErr.Timeout() {
		return true
	}
	return false
}
