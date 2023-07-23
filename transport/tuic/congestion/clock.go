package congestion

import "time"

// A Clock returns the current time
type Clock interface {
	Now() time.Time
}

// DefaultClock implements the Clock interface using the Go stdlib clock.
type DefaultClock struct {
	TimeFunc func() time.Time
}

var _ Clock = DefaultClock{}

// Now gets the current time
func (c DefaultClock) Now() time.Time {
	return c.TimeFunc()
}
