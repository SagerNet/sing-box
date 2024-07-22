package option

import (
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/uot"
)

type _UDPOverTCPOptions struct {
	Enabled bool  `json:"enabled,omitempty"`
	Version uint8 `json:"version,omitempty"`
}

type UDPOverTCPOptions _UDPOverTCPOptions

func (o UDPOverTCPOptions) MarshalJSON() ([]byte, error) {
	switch o.Version {
	case 0, uot.Version:
		return json.Marshal(o.Enabled)
	default:
		return json.Marshal(_UDPOverTCPOptions(o))
	}
}

func (o *UDPOverTCPOptions) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, &o.Enabled)
	if err == nil {
		return nil
	}
	return json.UnmarshalDisallowUnknownFields(bytes, (*_UDPOverTCPOptions)(o))
}
