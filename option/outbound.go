package option

import (
	"github.com/sagernet/sing-box/common/json"
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
)

type _Outbound struct {
	Type               string                     `json:"type"`
	Tag                string                     `json:"tag,omitempty"`
	DirectOptions      DirectOutboundOptions      `json:"-"`
	SocksOptions       SocksOutboundOptions       `json:"-"`
	HTTPOptions        HTTPOutboundOptions        `json:"-"`
	ShadowsocksOptions ShadowsocksOutboundOptions `json:"-"`
	VMessOptions       VMessOutboundOptions       `json:"-"`
	TrojanOptions      TrojanOutboundOptions      `json:"-"`
	WireGuardOptions   WireGuardOutboundOptions   `json:"-"`
	HysteriaOptions    HysteriaOutboundOptions    `json:"-"`
	TorOptions         TorOutboundOptions         `json:"-"`
	SSHOptions         SSHOutboundOptions         `json:"-"`
	ShadowTLSOptions   ShadowTLSOutboundOptions   `json:"-"`
	SelectorOptions    SelectorOutboundOptions    `json:"-"`
}

type Outbound _Outbound

func (h Outbound) MarshalJSON() ([]byte, error) {
	var v any
	switch h.Type {
	case C.TypeDirect:
		v = h.DirectOptions
	case C.TypeBlock, C.TypeDNS:
		v = nil
	case C.TypeSocks:
		v = h.SocksOptions
	case C.TypeHTTP:
		v = h.HTTPOptions
	case C.TypeShadowsocks:
		v = h.ShadowsocksOptions
	case C.TypeVMess:
		v = h.VMessOptions
	case C.TypeTrojan:
		v = h.TrojanOptions
	case C.TypeWireGuard:
		v = h.WireGuardOptions
	case C.TypeHysteria:
		v = h.HysteriaOptions
	case C.TypeTor:
		v = h.TorOptions
	case C.TypeSSH:
		v = h.SSHOptions
	case C.TypeShadowTLS:
		v = h.ShadowTLSOptions
	case C.TypeSelector:
		v = h.SelectorOptions
	default:
		return nil, E.New("unknown outbound type: ", h.Type)
	}
	return MarshallObjects((_Outbound)(h), v)
}

func (h *Outbound) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_Outbound)(h))
	if err != nil {
		return err
	}
	var v any
	switch h.Type {
	case C.TypeDirect:
		v = &h.DirectOptions
	case C.TypeBlock, C.TypeDNS:
		v = nil
	case C.TypeSocks:
		v = &h.SocksOptions
	case C.TypeHTTP:
		v = &h.HTTPOptions
	case C.TypeShadowsocks:
		v = &h.ShadowsocksOptions
	case C.TypeVMess:
		v = &h.VMessOptions
	case C.TypeTrojan:
		v = &h.TrojanOptions
	case C.TypeWireGuard:
		v = &h.WireGuardOptions
	case C.TypeHysteria:
		v = &h.HysteriaOptions
	case C.TypeTor:
		v = &h.TorOptions
	case C.TypeSSH:
		v = &h.SSHOptions
	case C.TypeShadowTLS:
		v = &h.ShadowTLSOptions
	case C.TypeSelector:
		v = &h.SelectorOptions
	default:
		return E.New("unknown outbound type: ", h.Type)
	}
	err = UnmarshallExcluded(bytes, (*_Outbound)(h), v)
	if err != nil {
		return E.Cause(err, "outbound options")
	}
	return nil
}

type DialerOptions struct {
	Detour         string         `json:"detour,omitempty"`
	BindInterface  string         `json:"bind_interface,omitempty"`
	BindAddress    *ListenAddress `json:"bind_address,omitempty"`
	ProtectPath    string         `json:"protect_path,omitempty"`
	RoutingMark    int            `json:"routing_mark,omitempty"`
	ReuseAddr      bool           `json:"reuse_addr,omitempty"`
	ConnectTimeout Duration       `json:"connect_timeout,omitempty"`
	TCPFastOpen    bool           `json:"tcp_fast_open,omitempty"`
}

type OutboundDialerOptions struct {
	DialerOptions
	DomainStrategy DomainStrategy `json:"domain_strategy,omitempty"`
	FallbackDelay  Duration       `json:"fallback_delay,omitempty"`
}

type ServerOptions struct {
	Server     string `json:"server"`
	ServerPort uint16 `json:"server_port"`
}

func (o ServerOptions) Build() M.Socksaddr {
	return M.ParseSocksaddrHostPort(o.Server, o.ServerPort)
}

type MultiplexOptions struct {
	Enabled        bool   `json:"enabled,omitempty"`
	Protocol       string `json:"protocol,omitempty"`
	MaxConnections int    `json:"max_connections,omitempty"`
	MinStreams     int    `json:"min_streams,omitempty"`
	MaxStreams     int    `json:"max_streams,omitempty"`
}
