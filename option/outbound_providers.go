package option

import (
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
)

type _OutboundProvider struct {
	Type             string                   `json:"type"`
	Path             string                   `json:"path"`
	Tag              string                   `json:"tag,omitempty"`
	OutboundOverride *OutboundOverrideOptions `json:"outbound_override,omitempty"`
	LocalOptions     LocalProviderOptions     `json:"-"`
	RemoteOptions    RemoteProviderOptions    `json:"-"`
	FilterOptions
}

type OutboundProvider _OutboundProvider

type OutboundOverrideOptions struct {
	TagPrefix string `json:"tag_prefix,omitempty"`
	TagSuffix string `json:"tag_suffix,omitempty"`
	*OverrideDialerOptions
}

type OverrideDialerOptions struct {
	Detour           *string         `json:"detour,omitempty"`
	BindInterface    *string         `json:"bind_interface,omitempty"`
	Inet4BindAddress *ListenAddress  `json:"inet4_bind_address,omitempty"`
	Inet6BindAddress *ListenAddress  `json:"inet6_bind_address,omitempty"`
	ProtectPath      *string         `json:"protect_path,omitempty"`
	RoutingMark      *uint32         `json:"routing_mark,omitempty"`
	ReuseAddr        *bool           `json:"reuse_addr,omitempty"`
	ConnectTimeout   *Duration       `json:"connect_timeout,omitempty"`
	TCPFastOpen      *bool           `json:"tcp_fast_open,omitempty"`
	TCPMultiPath     *bool           `json:"tcp_multi_path,omitempty"`
	UDPFragment      *bool           `json:"udp_fragment,omitempty"`
	DomainStrategy   *DomainStrategy `json:"domain_strategy,omitempty"`
	FallbackDelay    *Duration       `json:"fallback_delay,omitempty"`
}

type LocalProviderOptions struct {
	HealthcheckOptions
}

type RemoteProviderOptions struct {
	Url       string   `json:"download_url"`
	UserAgent string   `json:"download_ua,omitempty"`
	Interval  Duration `json:"download_interval,omitempty"`
	Detour    string   `json:"download_detour,omitempty"`
	HealthcheckOptions
}

func (h OutboundProvider) MarshalJSON() ([]byte, error) {
	var v any
	switch h.Type {
	case C.ProviderTypeLocal:
		v = h.LocalOptions
	case C.ProviderTypeRemote:
		v = h.RemoteOptions
	default:
		return nil, E.New("unknown provider type: ", h.Type)
	}
	return MarshallObjects((_OutboundProvider)(h), v)
}

func (h *OutboundProvider) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_OutboundProvider)(h))
	if err != nil {
		return err
	}
	var v any
	switch h.Type {
	case C.ProviderTypeLocal:
		v = &h.LocalOptions
	case C.ProviderTypeRemote:
		v = &h.RemoteOptions
	default:
		return E.New("unknown provider type: ", h.Type)
	}
	err = UnmarshallExcluded(bytes, (*_OutboundProvider)(h), v)
	if err != nil {
		return E.Cause(err, "provider options")
	}
	return nil
}

type HealthcheckOptions struct {
	EnableHealthcheck   bool     `json:"enable_healthcheck,omitempty"`
	HealthcheckUrl      string   `json:"healthcheck_url,omitempty"`
	HealthcheckInterval Duration `json:"healthcheck_interval,omitempty"`
}
