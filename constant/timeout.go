package constant

import "time"

const (
	DefaultTCPTimeout      = 5 * time.Second
	ReadPayloadTimeout     = 300 * time.Millisecond
	URLTestTimeout         = DefaultTCPTimeout
	DefaultURLTestInterval = 1 * time.Minute
)
