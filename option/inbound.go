package option

import (
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
)

type _Inbound struct {
	Type               string                    `json:"type"`
	Tag                string                    `json:"tag,omitempty"`
	TunOptions         TunInboundOptions         `json:"-"`
	RedirectOptions    RedirectInboundOptions    `json:"-"`
	TProxyOptions      TProxyInboundOptions      `json:"-"`
	DirectOptions      DirectInboundOptions      `json:"-"`
	SocksOptions       SocksInboundOptions       `json:"-"`
	HTTPOptions        HTTPMixedInboundOptions   `json:"-"`
	MixedOptions       HTTPMixedInboundOptions   `json:"-"`
	ShadowsocksOptions ShadowsocksInboundOptions `json:"-"`
	VMessOptions       VMessInboundOptions       `json:"-"`
	TrojanOptions      TrojanInboundOptions      `json:"-"`
	NaiveOptions       NaiveInboundOptions       `json:"-"`
	HysteriaOptions    HysteriaInboundOptions    `json:"-"`
	ShadowTLSOptions   ShadowTLSInboundOptions   `json:"-"`
	VLESSOptions       VLESSInboundOptions       `json:"-"`
	TUICOptions        TUICInboundOptions        `json:"-"`
	Hysteria2Options   Hysteria2InboundOptions   `json:"-"`
}

type Inbound _Inbound

func (h *Inbound) RawOptions() (any, error) {
	var rawOptionsPtr any
	switch h.Type {
	case C.TypeTun:
		rawOptionsPtr = &h.TunOptions
	case C.TypeRedirect:
		rawOptionsPtr = &h.RedirectOptions
	case C.TypeTProxy:
		rawOptionsPtr = &h.TProxyOptions
	case C.TypeDirect:
		rawOptionsPtr = &h.DirectOptions
	case C.TypeSOCKS:
		rawOptionsPtr = &h.SocksOptions
	case C.TypeHTTP:
		rawOptionsPtr = &h.HTTPOptions
	case C.TypeMixed:
		rawOptionsPtr = &h.MixedOptions
	case C.TypeShadowsocks:
		rawOptionsPtr = &h.ShadowsocksOptions
	case C.TypeVMess:
		rawOptionsPtr = &h.VMessOptions
	case C.TypeTrojan:
		rawOptionsPtr = &h.TrojanOptions
	case C.TypeNaive:
		rawOptionsPtr = &h.NaiveOptions
	case C.TypeHysteria:
		rawOptionsPtr = &h.HysteriaOptions
	case C.TypeShadowTLS:
		rawOptionsPtr = &h.ShadowTLSOptions
	case C.TypeVLESS:
		rawOptionsPtr = &h.VLESSOptions
	case C.TypeTUIC:
		rawOptionsPtr = &h.TUICOptions
	case C.TypeHysteria2:
		rawOptionsPtr = &h.Hysteria2Options
	case "":
		return nil, E.New("missing inbound type")
	default:
		return nil, E.New("unknown inbound type: ", h.Type)
	}
	return rawOptionsPtr, nil
}

func (h Inbound) MarshalJSON() ([]byte, error) {
	rawOptions, err := h.RawOptions()
	if err != nil {
		return nil, err
	}
	return MarshallObjects((_Inbound)(h), rawOptions)
}

func (h *Inbound) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_Inbound)(h))
	if err != nil {
		return err
	}
	rawOptions, err := h.RawOptions()
	if err != nil {
		return err
	}
	err = UnmarshallExcluded(bytes, (*_Inbound)(h), rawOptions)
	if err != nil {
		return err
	}
	return nil
}

type InboundOptions struct {
	SniffEnabled              bool           `json:"sniff,omitempty"`
	SniffOverrideDestination  bool           `json:"sniff_override_destination,omitempty"`
	SniffTimeout              Duration       `json:"sniff_timeout,omitempty"`
	DomainStrategy            DomainStrategy `json:"domain_strategy,omitempty"`
	UDPDisableDomainUnmapping bool           `json:"udp_disable_domain_unmapping,omitempty"`
}

type ListenOptions struct {
	Listen                      *ListenAddress `json:"listen,omitempty"`
	ListenPort                  uint16         `json:"listen_port,omitempty"`
	TCPFastOpen                 bool           `json:"tcp_fast_open,omitempty"`
	TCPMultiPath                bool           `json:"tcp_multi_path,omitempty"`
	UDPFragment                 *bool          `json:"udp_fragment,omitempty"`
	UDPFragmentDefault          bool           `json:"-"`
	UDPTimeout                  int64          `json:"udp_timeout,omitempty"`
	ProxyProtocol               bool           `json:"proxy_protocol,omitempty"`
	ProxyProtocolAcceptNoHeader bool           `json:"proxy_protocol_accept_no_header,omitempty"`
	Detour                      string         `json:"detour,omitempty"`
	InboundOptions
}

type ListenOptionsWrapper interface {
	TakeListenOptions() ListenOptions
	ReplaceListenOptions(options ListenOptions)
}

func (o *ListenOptions) TakeListenOptions() ListenOptions {
	return *o
}

func (o *ListenOptions) ReplaceListenOptions(options ListenOptions) {
	*o = options
}
