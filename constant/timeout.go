package constant

import "time"

const (
	TCPKeepAliveInitial        = 10 * time.Minute
	TCPKeepAliveInterval       = 75 * time.Second
	TCPConnectTimeout          = 5 * time.Second
	TCPTimeout                 = 15 * time.Second
	ReadPayloadTimeout         = 300 * time.Millisecond
	DNSTimeout                 = 10 * time.Second
	UDPTimeout                 = 5 * time.Minute
	DefaultURLTestInterval     = 3 * time.Minute
	DefaultURLTestIdleTimeout  = 30 * time.Minute
	StartTimeout               = 10 * time.Second
	StopTimeout                = 5 * time.Second
	FatalStopTimeout           = 10 * time.Second
	FakeIPMetadataSaveInterval = 10 * time.Second
	TLSFragmentFallbackDelay   = 500 * time.Millisecond
)

var PortProtocols = map[uint16]string{
	53:   ProtocolDNS,
	123:  ProtocolNTP,
	3478: ProtocolSTUN,
	443:  ProtocolQUIC,
}

var ProtocolTimeouts = map[string]time.Duration{
	ProtocolDNS:  10 * time.Second,
	ProtocolNTP:  10 * time.Second,
	ProtocolSTUN: 10 * time.Second,
	ProtocolQUIC: 30 * time.Second,
	ProtocolDTLS: 30 * time.Second,
}
