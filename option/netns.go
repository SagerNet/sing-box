package option

import (
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
)

type _NetworkNamespace struct {
	Type           string                         `json:"type,omitempty"`
	Tag            string                         `json:"tag"`
	DefaultOptions DefaultNetworkNamespaceOptions `json:"-"`
	UnshareOptions UnshareNetworkNamespaceOptions `json:"-"`
}

type NetworkNamespace _NetworkNamespace

func (o NetworkNamespace) MarshalJSON() ([]byte, error) {
	var v any
	switch o.Type {
	case C.NetNsTypeDefault:
		o.Type = ""
		v = o.DefaultOptions
	case C.NetNsTypeUnshare:
		v = o.UnshareOptions
	default:
		return nil, E.New("unknown network namespace type: ", o.Type)
	}
	return badjson.MarshallObjects((_NetworkNamespace)(o), v)
}

func (o *NetworkNamespace) UnmarshalJSON(content []byte) error {
	err := json.Unmarshal(content, (*_NetworkNamespace)(o))
	if err != nil {
		return err
	}
	var v any
	switch o.Type {
	case "", C.NetNsTypeDefault:
		o.Type = C.NetNsTypeDefault
		v = &o.DefaultOptions
	case C.NetNsTypeUnshare:
		v = &o.UnshareOptions
	default:
		return E.New("unknown network namespace type: ", o.Type)
	}
	return badjson.UnmarshallExcluded(content, (*_NetworkNamespace)(o), v)
}

type DefaultNetworkNamespaceOptions struct {
	Path string `json:"path"`
}

type UnshareNetworkNamespaceOptions struct {
	PidFile string `json:"pid_file,omitempty"`
}
