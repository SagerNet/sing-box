package option

import (
	"context"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
)

type DirectInboundOptions struct {
	ListenOptions
	Network         NetworkList `json:"network,omitempty"`
	OverrideAddress string      `json:"override_address,omitempty"`
	OverridePort    uint16      `json:"override_port,omitempty"`
}

type _DirectOutboundOptions struct {
	DialerOptions
	// Deprecated: Use Route Action instead
	OverrideAddress string `json:"override_address,omitempty"`
	// Deprecated: Use Route Action instead
	OverridePort uint16 `json:"override_port,omitempty"`
	// Deprecated: removed
	ProxyProtocol uint8 `json:"proxy_protocol,omitempty"`
}

type DirectOutboundOptions _DirectOutboundOptions

func (d *DirectOutboundOptions) UnmarshalJSONContext(ctx context.Context, content []byte) error {
	err := json.UnmarshalDisallowUnknownFields(content, (*_DirectOutboundOptions)(d))
	if err != nil {
		return err
	}
	//nolint:staticcheck
	if d.OverrideAddress != "" || d.OverridePort != 0 {
		return E.New("destination override fields in direct outbound are deprecated in sing-box 1.11.0 and removed in sing-box 1.13.0, use route options instead")
	}
	return nil
}
