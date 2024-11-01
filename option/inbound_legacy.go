package option

import (
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
)

type _LegacyInbound struct {
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

type LegacyInbound _LegacyInbound

func (h *LegacyInbound) RawOptions() (any, error) {
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

func (h LegacyInbound) MarshalJSON() ([]byte, error) {
	rawOptions, err := h.RawOptions()
	if err != nil {
		return nil, err
	}
	return badjson.MarshallObjects((_LegacyInbound)(h), rawOptions)
}

func (h *LegacyInbound) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_LegacyInbound)(h))
	if err != nil {
		return err
	}
	rawOptions, err := h.RawOptions()
	if err != nil {
		return err
	}
	err = badjson.UnmarshallExcluded(bytes, (*_LegacyInbound)(h), rawOptions)
	if err != nil {
		return err
	}
	return nil
}
