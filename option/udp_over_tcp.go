package option

import (
	"reflect"

	"github.com/sagernet/sing-box/schema"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/uot"
)

type _UDPOverTCPOptions struct {
	Enabled bool  `json:"enabled,omitempty"`
	Version uint8 `json:"version,omitempty" enum:"1,2"`
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

func (o UDPOverTCPOptions) DescribeSchema(builder schema.Builder) (*schema.Node, error) {
	objectForm := schema.StrictObject()
	err := builder.FlattenStruct(objectForm, reflect.TypeFor[UDPOverTCPOptions]())
	if err != nil {
		return nil, err
	}
	return schema.AnyOf(schema.BooleanNode(), objectForm), nil
}
