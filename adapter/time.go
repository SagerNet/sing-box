package adapter

import "time"

type TimeService interface {
	SimpleLifecycle
	TimeFunc() func() time.Time
}
