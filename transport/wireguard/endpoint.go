package wireguard

import (
	"net/netip"

	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/wireguard-go/conn"
)

var _ conn.Endpoint = (*Endpoint)(nil)

type Endpoint M.Socksaddr

func (e Endpoint) ClearSrc() {
}

func (e Endpoint) SrcToString() string {
	return ""
}

func (e Endpoint) DstToString() string {
	return (M.Socksaddr)(e).String()
}

func (e Endpoint) DstToBytes() []byte {
	b, _ := (M.Socksaddr)(e).AddrPort().MarshalBinary()
	return b
}

func (e Endpoint) DstIP() netip.Addr {
	return (M.Socksaddr)(e).Addr
}

func (e Endpoint) SrcIP() netip.Addr {
	return netip.Addr{}
}
