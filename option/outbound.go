package option

import (
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	M "github.com/sagernet/sing/common/metadata"
)

type _Outbound struct {
	Type                string                      `json:"type"`
	Tag                 string                      `json:"tag,omitempty"`
	DirectOptions       DirectOutboundOptions       `json:"-"`
	SocksOptions        SocksOutboundOptions        `json:"-"`
	HTTPOptions         HTTPOutboundOptions         `json:"-"`
	ShadowsocksOptions  ShadowsocksOutboundOptions  `json:"-"`
	VMessOptions        VMessOutboundOptions        `json:"-"`
	TrojanOptions       TrojanOutboundOptions       `json:"-"`
	WireGuardOptions    WireGuardOutboundOptions    `json:"-"`
	HysteriaOptions     HysteriaOutboundOptions     `json:"-"`
	TorOptions          TorOutboundOptions          `json:"-"`
	SSHOptions          SSHOutboundOptions          `json:"-"`
	ShadowTLSOptions    ShadowTLSOutboundOptions    `json:"-"`
	ShadowsocksROptions ShadowsocksROutboundOptions `json:"-"`
	VLESSOptions        VLESSOutboundOptions        `json:"-"`
	TUICOptions         TUICOutboundOptions         `json:"-"`
	Hysteria2Options    Hysteria2OutboundOptions    `json:"-"`
	SelectorOptions     SelectorOutboundOptions     `json:"-"`
	URLTestOptions      URLTestOutboundOptions      `json:"-"`
}

type Outbound _Outbound

func (h *Outbound) RawOptions() (any, error) {
	var rawOptionsPtr any
	switch h.Type {
	case C.TypeDirect:
		rawOptionsPtr = &h.DirectOptions
	case C.TypeBlock, C.TypeDNS:
		rawOptionsPtr = nil
	case C.TypeSOCKS:
		rawOptionsPtr = &h.SocksOptions
	case C.TypeHTTP:
		rawOptionsPtr = &h.HTTPOptions
	case C.TypeShadowsocks:
		rawOptionsPtr = &h.ShadowsocksOptions
	case C.TypeVMess:
		rawOptionsPtr = &h.VMessOptions
	case C.TypeTrojan:
		rawOptionsPtr = &h.TrojanOptions
	case C.TypeWireGuard:
		rawOptionsPtr = &h.WireGuardOptions
	case C.TypeHysteria:
		rawOptionsPtr = &h.HysteriaOptions
	case C.TypeTor:
		rawOptionsPtr = &h.TorOptions
	case C.TypeSSH:
		rawOptionsPtr = &h.SSHOptions
	case C.TypeShadowTLS:
		rawOptionsPtr = &h.ShadowTLSOptions
	case C.TypeShadowsocksR:
		rawOptionsPtr = &h.ShadowsocksROptions
	case C.TypeVLESS:
		rawOptionsPtr = &h.VLESSOptions
	case C.TypeTUIC:
		rawOptionsPtr = &h.TUICOptions
	case C.TypeHysteria2:
		rawOptionsPtr = &h.Hysteria2Options
	case C.TypeSelector:
		rawOptionsPtr = &h.SelectorOptions
	case C.TypeURLTest:
		rawOptionsPtr = &h.URLTestOptions
	case "":
		return nil, E.New("missing outbound type")
	default:
		return nil, E.New("unknown outbound type: ", h.Type)
	}
	return rawOptionsPtr, nil
}

func (h *Outbound) MarshalJSON() ([]byte, error) {
	rawOptions, err := h.RawOptions()
	if err != nil {
		return nil, err
	}
	return MarshallObjects((*_Outbound)(h), rawOptions)
}

func (h *Outbound) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_Outbound)(h))
	if err != nil {
		return err
	}
	rawOptions, err := h.RawOptions()
	if err != nil {
		return err
	}
	err = UnmarshallExcluded(bytes, (*_Outbound)(h), rawOptions)
	if err != nil {
		return err
	}
	return nil
}

func (h *Outbound) Port() int {
	var port uint16
	switch h.Type {
	case C.TypeSOCKS:
		port = h.SocksOptions.ServerPort
	case C.TypeHTTP:
		port = h.HTTPOptions.ServerPort
	case C.TypeShadowsocks:
		port = h.ShadowsocksOptions.ServerPort
	case C.TypeVMess:
		port = h.VMessOptions.ServerPort
	case C.TypeTrojan:
		port = h.TrojanOptions.ServerPort
	case C.TypeWireGuard:
		port = h.WireGuardOptions.ServerPort
	case C.TypeHysteria:
		port = h.HysteriaOptions.ServerPort
	case C.TypeSSH:
		port = h.SSHOptions.ServerPort
	case C.TypeShadowTLS:
		port = h.ShadowTLSOptions.ServerPort
	case C.TypeShadowsocksR:
		port = h.ShadowsocksROptions.ServerPort
	case C.TypeVLESS:
		port = h.VLESSOptions.ServerPort
	case C.TypeTUIC:
		port = h.TUICOptions.ServerPort
	case C.TypeHysteria2:
		port = h.Hysteria2Options.ServerPort
	}
	return int(port)
}

type DialerOptionsWrapper interface {
	TakeDialerOptions() DialerOptions
	ReplaceDialerOptions(options DialerOptions)
}

type DialerOptions struct {
	Detour              string         `json:"detour,omitempty"`
	BindInterface       string         `json:"bind_interface,omitempty"`
	Inet4BindAddress    *ListenAddress `json:"inet4_bind_address,omitempty"`
	Inet6BindAddress    *ListenAddress `json:"inet6_bind_address,omitempty"`
	ProtectPath         string         `json:"protect_path,omitempty"`
	RoutingMark         uint32         `json:"routing_mark,omitempty"`
	ReuseAddr           bool           `json:"reuse_addr,omitempty"`
	ConnectTimeout      Duration       `json:"connect_timeout,omitempty"`
	TCPFastOpen         bool           `json:"tcp_fast_open,omitempty"`
	TCPMultiPath        bool           `json:"tcp_multi_path,omitempty"`
	UDPFragment         *bool          `json:"udp_fragment,omitempty"`
	UDPFragmentDefault  bool           `json:"-"`
	DomainStrategy      DomainStrategy `json:"domain_strategy,omitempty"`
	FallbackDelay       Duration       `json:"fallback_delay,omitempty"`
	IsWireGuardListener bool           `json:"-"`
}

func (o *DialerOptions) TakeDialerOptions() DialerOptions {
	return *o
}

func (o *DialerOptions) ReplaceDialerOptions(options DialerOptions) {
	*o = options
}

type ServerOptionsWrapper interface {
	TakeServerOptions() ServerOptions
	ReplaceServerOptions(options ServerOptions)
}

type ServerOptions struct {
	Server     string `json:"server"`
	ServerPort uint16 `json:"server_port"`
}

func (o ServerOptions) Build() M.Socksaddr {
	return M.ParseSocksaddrHostPort(o.Server, o.ServerPort)
}

func (o *ServerOptions) TakeServerOptions() ServerOptions {
	return *o
}

func (o *ServerOptions) ReplaceServerOptions(options ServerOptions) {
	*o = options
}
