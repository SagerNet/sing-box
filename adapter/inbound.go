package adapter

import (
	"net/netip"

	M "github.com/sagernet/sing/common/metadata"
)

type Inbound interface {
	Service
	Type() string
	Tag() string
}

type InboundContext struct {
	Source      netip.AddrPort
	Destination M.Socksaddr
	Inbound     string
	Network     string
	Protocol    string
	Domain      string
}
