package tls

import (
	"time"

	"github.com/sagernet/sing/common/ntp"
)

type TimeServiceWrapper struct {
	ntp.TimeService
}

func (w *TimeServiceWrapper) TimeFunc() func() time.Time {
	if w.TimeService == nil {
		return nil
	}
	return w.TimeService.TimeFunc()
}

func (w *TimeServiceWrapper) Upstream() any {
	return w.TimeService
}
