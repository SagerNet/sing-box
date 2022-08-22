package option

import (
	"github.com/sagernet/sing-box/common/json"
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
)

type _V2RayInboundTransportOptions struct {
	Type        string           `json:"type,omitempty"`
	GRPCOptions V2RayGRPCOptions `json:"-"`
}

type V2RayInboundTransportOptions _V2RayOutboundTransportOptions

func (o V2RayInboundTransportOptions) MarshalJSON() ([]byte, error) {
	var v any
	switch o.Type {
	case "":
		return nil, nil
	case C.V2RayTransportTypeGRPC:
		v = o.GRPCOptions
	default:
		return nil, E.New("unknown transport type: " + o.Type)
	}
	return MarshallObjects((_V2RayOutboundTransportOptions)(o), v)
}

func (o *V2RayInboundTransportOptions) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_V2RayOutboundTransportOptions)(o))
	if err != nil {
		return err
	}
	var v any
	switch o.Type {
	case C.V2RayTransportTypeGRPC:
		v = &o.GRPCOptions
	default:
		return E.New("unknown transport type: " + o.Type)
	}
	err = UnmarshallExcluded(bytes, (*_V2RayOutboundTransportOptions)(o), v)
	if err != nil {
		return E.Cause(err, "vmess transport options")
	}
	return nil
}

type _V2RayOutboundTransportOptions struct {
	Type        string           `json:"type,omitempty"`
	GRPCOptions V2RayGRPCOptions `json:"-"`
}

type V2RayOutboundTransportOptions _V2RayOutboundTransportOptions

func (o V2RayOutboundTransportOptions) MarshalJSON() ([]byte, error) {
	var v any
	switch o.Type {
	case "":
		return nil, nil
	case C.V2RayTransportTypeGRPC:
		v = o.GRPCOptions
	default:
		return nil, E.New("unknown transport type: " + o.Type)
	}
	return MarshallObjects((_V2RayOutboundTransportOptions)(o), v)
}

func (o *V2RayOutboundTransportOptions) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_V2RayOutboundTransportOptions)(o))
	if err != nil {
		return err
	}
	var v any
	switch o.Type {
	case C.V2RayTransportTypeGRPC:
		v = &o.GRPCOptions
	default:
		return E.New("unknown transport type: " + o.Type)
	}
	err = UnmarshallExcluded(bytes, (*_V2RayOutboundTransportOptions)(o), v)
	if err != nil {
		return E.Cause(err, "vmess transport options")
	}
	return nil
}

type V2RayGRPCOptions struct {
	ServiceName string `json:"service_name,omitempty"`
}
