package option

import (
	"github.com/sagernet/sing-box/common/json"
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
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
}

type Inbound _Inbound

func (h Inbound) MarshalJSON() ([]byte, error) {
	var v any
	switch h.Type {
	case C.TypeTun:
		v = h.TunOptions
	case C.TypeRedirect:
		v = h.RedirectOptions
	case C.TypeTProxy:
		v = h.TProxyOptions
	case C.TypeDirect:
		v = h.DirectOptions
	case C.TypeSocks:
		v = h.SocksOptions
	case C.TypeHTTP:
		v = h.HTTPOptions
	case C.TypeMixed:
		v = h.MixedOptions
	case C.TypeShadowsocks:
		v = h.ShadowsocksOptions
	case C.TypeVMess:
		v = h.VMessOptions
	case C.TypeTrojan:
		v = h.TrojanOptions
	case C.TypeNaive:
		v = h.NaiveOptions
	case C.TypeHysteria:
		v = h.HysteriaOptions
	default:
		return nil, E.New("unknown inbound type: ", h.Type)
	}
	return MarshallObjects((_Inbound)(h), v)
}

func (h *Inbound) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_Inbound)(h))
	if err != nil {
		return err
	}
	var v any
	switch h.Type {
	case C.TypeTun:
		v = &h.TunOptions
	case C.TypeRedirect:
		v = &h.RedirectOptions
	case C.TypeTProxy:
		v = &h.TProxyOptions
	case C.TypeDirect:
		v = &h.DirectOptions
	case C.TypeSocks:
		v = &h.SocksOptions
	case C.TypeHTTP:
		v = &h.HTTPOptions
	case C.TypeMixed:
		v = &h.MixedOptions
	case C.TypeShadowsocks:
		v = &h.ShadowsocksOptions
	case C.TypeVMess:
		v = &h.VMessOptions
	case C.TypeTrojan:
		v = &h.TrojanOptions
	case C.TypeNaive:
		v = &h.NaiveOptions
	case C.TypeHysteria:
		v = &h.HysteriaOptions
	default:
		return E.New("unknown inbound type: ", h.Type)
	}
	err = UnmarshallExcluded(bytes, (*_Inbound)(h), v)
	if err != nil {
		return E.Cause(err, "inbound options")
	}
	return nil
}

type InboundOptions struct {
	SniffEnabled             bool           `json:"sniff,omitempty"`
	SniffOverrideDestination bool           `json:"sniff_override_destination,omitempty"`
	DomainStrategy           DomainStrategy `json:"domain_strategy,omitempty"`
}

type ListenOptions struct {
	Listen        ListenAddress `json:"listen"`
	ListenPort    uint16        `json:"listen_port,omitempty"`
	TCPFastOpen   bool          `json:"tcp_fast_open,omitempty"`
	UDPTimeout    int64         `json:"udp_timeout,omitempty"`
	ProxyProtocol bool          `json:"proxy_protocol,omitempty"`
	InboundOptions
}
