package wireguard

import (
	"net/netip"

	"github.com/sagernet/wireguard-go/conn"
)

var _ conn.Endpoint = (*Endpoint)(nil)

type Endpoint netip.AddrPort

func (e Endpoint) ClearSrc() {
}

func (e Endpoint) SrcToString() string {
	return ""
}

func (e Endpoint) DstToString() string {
	return (netip.AddrPort)(e).String()
}

func (e Endpoint) DstToBytes() []byte {
	b, _ := (netip.AddrPort)(e).MarshalBinary()
	return b
}

func (e Endpoint) DstIP() netip.Addr {
	return (netip.AddrPort)(e).Addr()
}

func (e Endpoint) SrcIP() netip.Addr {
	return netip.Addr{}
}
