package constant

import "time"

const (
	TCPTimeout         = 5 * time.Second
	ReadPayloadTimeout = 300 * time.Millisecond
	DNSTimeout         = 10 * time.Second
	QUICTimeout        = 30 * time.Second
	STUNTimeout        = 15 * time.Second
	UDPTimeout         = 5 * time.Minute
)
