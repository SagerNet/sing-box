package option

import (
	"encoding/json"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	E "github.com/sagernet/sing/common/exceptions"
)

type _Inbound struct {
	Tag                string                    `json:"tag,omitempty"`
	Type               string                    `json:"type"`
	DirectOptions      DirectInboundOptions      `json:"-"`
	SocksOptions       SimpleInboundOptions      `json:"-"`
	HTTPOptions        SimpleInboundOptions      `json:"-"`
	MixedOptions       SimpleInboundOptions      `json:"-"`
	ShadowsocksOptions ShadowsocksInboundOptions `json:"-"`
}

type Inbound _Inbound

func (h Inbound) Equals(other Inbound) bool {
	return h.Type == other.Type &&
		h.Tag == other.Tag &&
		common.Equals(h.DirectOptions, other.DirectOptions) &&
		common.Equals(h.SocksOptions, other.SocksOptions) &&
		common.Equals(h.HTTPOptions, other.HTTPOptions) &&
		common.Equals(h.MixedOptions, other.MixedOptions) &&
		common.Equals(h.ShadowsocksOptions, other.ShadowsocksOptions)
}

func (h Inbound) MarshalJSON() ([]byte, error) {
	var v any
	switch h.Type {
	case "direct":
		v = h.DirectOptions
	case "socks":
		v = h.SocksOptions
	case "http":
		v = h.HTTPOptions
	case "mixed":
		v = h.MixedOptions
	case "shadowsocks":
		v = h.ShadowsocksOptions
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
	case "direct":
		v = &h.DirectOptions
	case "socks":
		v = &h.SocksOptions
	case "http":
		v = &h.HTTPOptions
	case "mixed":
		v = &h.MixedOptions
	case "shadowsocks":
		v = &h.ShadowsocksOptions
	default:
		return nil
	}
	return json.Unmarshal(bytes, v)
}

type ListenOptions struct {
	Listen      ListenAddress `json:"listen"`
	Port        uint16        `json:"listen_port"`
	TCPFastOpen bool          `json:"tcp_fast_open,omitempty"`
	UDPTimeout  int64         `json:"udp_timeout,omitempty"`
}

type SimpleInboundOptions struct {
	ListenOptions
	Users []auth.User `json:"users,omitempty"`
}

func (o SimpleInboundOptions) Equals(other SimpleInboundOptions) bool {
	return o.ListenOptions == other.ListenOptions &&
		common.ComparableSliceEquals(o.Users, other.Users)
}

type DirectInboundOptions struct {
	ListenOptions
	Network         NetworkList `json:"network,omitempty"`
	OverrideAddress string      `json:"override_address,omitempty"`
	OverridePort    uint16      `json:"override_port,omitempty"`
}

func (o DirectInboundOptions) Equals(other DirectInboundOptions) bool {
	return o.ListenOptions == other.ListenOptions &&
		common.ComparableSliceEquals(o.Network, other.Network) &&
		o.OverrideAddress == other.OverrideAddress &&
		o.OverridePort == other.OverridePort
}

type ShadowsocksInboundOptions struct {
	ListenOptions
	Network  NetworkList `json:"network,omitempty"`
	Method   string      `json:"method"`
	Password string      `json:"password"`
}

func (o ShadowsocksInboundOptions) Equals(other ShadowsocksInboundOptions) bool {
	return o.ListenOptions == other.ListenOptions &&
		common.ComparableSliceEquals(o.Network, other.Network) &&
		o.Method == other.Method &&
		o.Password == other.Password
}
