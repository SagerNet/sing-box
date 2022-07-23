package option

import (
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/goccy/go-json"
)

type _Outbound struct {
	Type               string                     `json:"type"`
	Tag                string                     `json:"tag,omitempty"`
	DirectOptions      DirectOutboundOptions      `json:"-"`
	SocksOptions       SocksOutboundOptions       `json:"-"`
	HTTPOptions        HTTPOutboundOptions        `json:"-"`
	ShadowsocksOptions ShadowsocksOutboundOptions `json:"-"`
	VMessOptions       VMessOutboundOptions       `json:"-"`
	SelectorOptions    SelectorOutboundOptions    `json:"-"`
	URLTestOptions     URLTestOutboundOptions     `json:"-"`
}

type Outbound _Outbound

func (h Outbound) Equals(other Outbound) bool {
	return h.Type == other.Type &&
		h.Tag == other.Tag &&
		h.DirectOptions == other.DirectOptions &&
		h.SocksOptions == other.SocksOptions &&
		h.HTTPOptions == other.HTTPOptions &&
		h.ShadowsocksOptions == other.ShadowsocksOptions &&
		h.VMessOptions == other.VMessOptions &&
		common.Equals(h.SelectorOptions, other.SelectorOptions) &&
		common.Equals(h.URLTestOptions, other.URLTestOptions)
}

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
	case C.TypeSelector:
		v = h.SelectorOptions
	case C.TypeURLTest:
		v = h.URLTestOptions
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
	case C.TypeSelector:
		v = &h.SelectorOptions
	case C.TypeURLTest:
		v = &h.URLTestOptions
	default:
		return nil
	}
	err = UnmarshallExcluded(bytes, (*_Outbound)(h), v)
	if err != nil {
		return E.Cause(err, "outbound options")
	}
	return nil
}

type DialerOptions struct {
	Detour         string   `json:"detour,omitempty"`
	BindInterface  string   `json:"bind_interface,omitempty"`
	ProtectPath    string   `json:"protect_path,omitempty"`
	RoutingMark    int      `json:"routing_mark,omitempty"`
	ReuseAddr      bool     `json:"reuse_addr,omitempty"`
	ConnectTimeout Duration `json:"connect_timeout,omitempty"`
	TCPFastOpen    bool     `json:"tcp_fast_open,omitempty"`
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

type DirectOutboundOptions struct {
	OutboundDialerOptions
	OverrideAddress string `json:"override_address,omitempty"`
	OverridePort    uint16 `json:"override_port,omitempty"`
}

type SocksOutboundOptions struct {
	OutboundDialerOptions
	ServerOptions
	Version  string      `json:"version,omitempty"`
	Username string      `json:"username,omitempty"`
	Password string      `json:"password,omitempty"`
	Network  NetworkList `json:"network,omitempty"`
}

type HTTPOutboundOptions struct {
	OutboundDialerOptions
	ServerOptions
	Username   string              `json:"username,omitempty"`
	Password   string              `json:"password,omitempty"`
	TLSOptions *OutboundTLSOptions `json:"tls,omitempty"`
}

type OutboundTLSOptions struct {
	Enabled    bool   `json:"enabled,omitempty"`
	DisableSNI bool   `json:"disable_sni,omitempty"`
	ServerName string `json:"server_name,omitempty"`
	Insecure   bool   `json:"insecure,omitempty"`
}

type ShadowsocksOutboundOptions struct {
	OutboundDialerOptions
	ServerOptions
	Method   string      `json:"method"`
	Password string      `json:"password"`
	Network  NetworkList `json:"network,omitempty"`
}

type VMessOutboundOptions struct {
	OutboundDialerOptions
	ServerOptions
	UUID                string              `json:"uuid"`
	Security            string              `json:"security"`
	AlterId             int                 `json:"alter_id,omitempty"`
	GlobalPadding       bool                `json:"global_padding,omitempty"`
	AuthenticatedLength bool                `json:"authenticated_length,omitempty"`
	Network             NetworkList         `json:"network,omitempty"`
	TLSOptions          *OutboundTLSOptions `json:"tls,omitempty"`
}

type SelectorOutboundOptions struct {
	Outbounds []string `json:"outbounds"`
	Default   string   `json:"default,omitempty"`
}

func (o SelectorOutboundOptions) Equals(other SelectorOutboundOptions) bool {
	return common.ComparableSliceEquals(o.Outbounds, other.Outbounds) &&
		o.Default == other.Default
}

type URLTestOutboundOptions struct {
	Outbounds []string `json:"outbounds"`
	URL       string   `json:"url,omitempty"`
	Interval  Duration `json:"interval,omitempty"`
	Tolerance uint16   `json:"tolerance,omitempty"`
}

func (o URLTestOutboundOptions) Equals(other URLTestOutboundOptions) bool {
	return common.ComparableSliceEquals(o.Outbounds, other.Outbounds) &&
		o.URL == other.URL &&
		o.Interval == other.Interval &&
		o.Tolerance == other.Tolerance
}
