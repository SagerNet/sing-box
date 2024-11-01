package option

import (
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
)

type _LegacyOutbound struct {
	Type                string                      `json:"type"`
	Tag                 string                      `json:"tag,omitempty"`
	DirectOptions       DirectOutboundOptions       `json:"-"`
	SocksOptions        SOCKSOutboundOptions        `json:"-"`
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

type LegacyOutbound _LegacyOutbound

func (h *LegacyOutbound) RawOptions() (any, error) {
	var rawOptionsPtr any
	switch h.Type {
	case C.TypeDirect:
		rawOptionsPtr = &h.DirectOptions
	case C.TypeBlock, C.TypeDNS:
		rawOptionsPtr = new(StubOptions)
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

func (h *LegacyOutbound) MarshalJSON() ([]byte, error) {
	rawOptions, err := h.RawOptions()
	if err != nil {
		return nil, err
	}
	return badjson.MarshallObjects((*_LegacyOutbound)(h), rawOptions)
}

func (h *LegacyOutbound) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_LegacyOutbound)(h))
	if err != nil {
		return err
	}
	rawOptions, err := h.RawOptions()
	if err != nil {
		return err
	}
	err = badjson.UnmarshallExcluded(bytes, (*_LegacyOutbound)(h), rawOptions)
	if err != nil {
		return err
	}
	return nil
}
