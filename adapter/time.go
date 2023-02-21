package adapter

import "time"

type TimeService interface {
	Service
	TimeFunc() func() time.Time
}
