package tls

import (
	"time"

	"github.com/sagernet/sing/common/ntp"
)

type TimeServiceWrapper struct {
	ntp.TimeService
}

func (w *TimeServiceWrapper) TimeFunc() func() time.Time {
	return func() time.Time {
		if w.TimeService != nil {
			return w.TimeService.TimeFunc()()
		} else {
			return time.Now()
		}
	}
}

func (w *TimeServiceWrapper) Upstream() any {
	return w.TimeService
}
