package option

import (
	E "github.com/sagernet/sing/common/exceptions"
)

type VMessInboundOptions struct {
	ListenOptions
	Users []VMessUser        `json:"users,omitempty"`
	TLS   *InboundTLSOptions `json:"tls,omitempty"`
}

type VMessUser struct {
	Name string `json:"name"`
	UUID string `json:"uuid"`
}

type VMessOutboundOptions struct {
	OutboundDialerOptions
	ServerOptions
	UUID                string                         `json:"uuid"`
	Security            string                         `json:"security"`
	AlterId             int                            `json:"alter_id,omitempty"`
	GlobalPadding       bool                           `json:"global_padding,omitempty"`
	AuthenticatedLength bool                           `json:"authenticated_length,omitempty"`
	Network             NetworkList                    `json:"network,omitempty"`
	TLSOptions          *OutboundTLSOptions            `json:"tls,omitempty"`
	TransportOptions    *VMessOutboundTransportOptions `json:"transport,omitempty"`
}

type _VMessOutboundTransportOptions struct {
	Type        string                    `json:"network,omitempty"`
	HTTPOptions *VMessOutboundHTTPOptions `json:"-"`
}

type VMessOutboundTransportOptions _VMessOutboundTransportOptions

func (o VMessOutboundTransportOptions) MarshalJSON() ([]byte, error) {
	var v any
	switch o.Type {
	case "http":
		v = o.HTTPOptions
	default:
		return nil, E.New("unknown transport type: ", o.Type)
	}
	return MarshallObjects(_VMessOutboundTransportOptions(o), v)
}

type VMessOutboundHTTPOptions struct {
	Method  string            `json:"method,omitempty"`
	Host    string            `json:"host,omitempty"`
	Path    []string          `proxy:"path,omitempty"`
	Headers map[string]string `proxy:"headers,omitempty"`
}
