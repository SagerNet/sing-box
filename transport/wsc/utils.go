package wsc

import "time"

func nowns() int64 {
	return time.Now().UnixNano()
}
