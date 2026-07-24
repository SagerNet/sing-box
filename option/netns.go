package option

import (
	"reflect"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/schema"
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

func (o NetworkNamespace) DescribeSchema(builder schema.Builder) (*schema.Node, error) {
	return builder.Define("NetworkNamespace", func() (*schema.Node, error) {
		return schema.DiscriminatedUnion(builder, "type", false, []schema.UnionVariant{
			{Value: C.NetNsTypeDefault, StructType: reflect.TypeFor[DefaultNetworkNamespaceOptions](), TypeOptional: true},
			{Value: C.NetNsTypeUnshare, StructType: reflect.TypeFor[UnshareNetworkNamespaceOptions]()},
		}, func(variant *schema.Node) error {
			variant.Properties.Put("tag", schema.StringNode())
			variant.Required = append(variant.Required, "tag")
			return nil
		})
	})
}

type DefaultNetworkNamespaceOptions struct {
	Path string `json:"path"`
}

type UnshareNetworkNamespaceOptions struct {
	PidFile string `json:"pid_file,omitempty"`
}
