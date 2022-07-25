package constant

import "time"

const (
	TCPTimeout             = 5 * time.Second
	TCPKeepAlivePeriod     = 30 * time.Second
	ReadPayloadTimeout     = 300 * time.Millisecond
	URLTestTimeout         = TCPTimeout
	DefaultURLTestInterval = 1 * time.Minute
	DNSTimeout             = 10 * time.Second
	QUICTimeout            = 30 * time.Second
	STUNTimeout            = 15 * time.Second
)
