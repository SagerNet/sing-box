package schema

import (
	"github.com/sagernet/sing/common/json/badjson"
)

type Node struct {
	SchemaURI             string                           `json:"$schema,omitempty"`
	ID                    string                           `json:"$id,omitempty"`
	Ref                   string                           `json:"$ref,omitempty"`
	Type                  any                              `json:"type,omitempty"`
	Const                 any                              `json:"const,omitempty"`
	Enum                  []any                            `json:"enum,omitempty"`
	Pattern               string                           `json:"pattern,omitempty"`
	Minimum               *int64                           `json:"minimum,omitempty"`
	Maximum               *uint64                          `json:"maximum,omitempty"`
	Items                 *Node                            `json:"items,omitempty"`
	Properties            *badjson.TypedMap[string, *Node] `json:"properties,omitempty"`
	Required              []string                         `json:"required,omitempty"`
	PropertyNames         *Node                            `json:"propertyNames,omitempty"`
	AdditionalProperties  any                              `json:"additionalProperties,omitempty"`
	UnevaluatedProperties any                              `json:"unevaluatedProperties,omitempty"`
	AllOf                 []*Node                          `json:"allOf,omitempty"`
	AnyOf                 []*Node                          `json:"anyOf,omitempty"`
	OneOf                 []*Node                          `json:"oneOf,omitempty"`
	Deprecated            bool                             `json:"deprecated,omitempty"`
	Examples              []any                            `json:"examples,omitempty"`
	TagReference          string                           `json:"x-tag-reference,omitempty"`
	Defs                  *badjson.TypedMap[string, *Node] `json:"$defs,omitempty"`
}

func StrictObject() *Node {
	return &Node{
		Type:                 "object",
		Properties:           new(badjson.TypedMap[string, *Node]),
		AdditionalProperties: false,
	}
}

func LooseObject() *Node {
	return &Node{
		Type:       "object",
		Properties: new(badjson.TypedMap[string, *Node]),
	}
}

func StringNode() *Node {
	return &Node{Type: "string"}
}

func TagReferenceNode(kind string) *Node {
	return &Node{Type: "string", TagReference: kind}
}

func BooleanNode() *Node {
	return &Node{Type: "boolean"}
}

func IntegerNode() *Node {
	return &Node{Type: "integer"}
}

func UnsignedNode(bits int) *Node {
	minimumValue := int64(0)
	node := &Node{Type: "integer", Minimum: &minimumValue}
	if bits < 64 {
		maximumValue := uint64(1)<<bits - 1
		node.Maximum = &maximumValue
	}
	return node
}

const durationPattern = `^[-+]?(((\d+(\.\d*)?|\.\d+)(ns|us|µs|μs|ms|s|m|h|d))+|0)$`

func DurationNode() *Node {
	return &Node{Type: "string", Pattern: durationPattern}
}

const (
	ipv4Octet     = `(?:25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])`
	ipv4Address   = ipv4Octet + `(?:\.` + ipv4Octet + `){3}`
	ipv6Group     = `[0-9A-Fa-f]{1,4}`
	ipv6Low32Bits = `(?:` + ipv6Group + `:` + ipv6Group + `|` + ipv4Address + `)`
	ipv6Address   = `(?:` +
		`(?:` + ipv6Group + `:){6}` + ipv6Low32Bits +
		`|::(?:` + ipv6Group + `:){5}` + ipv6Low32Bits +
		`|(?:` + ipv6Group + `)?::(?:` + ipv6Group + `:){4}` + ipv6Low32Bits +
		`|(?:(?:` + ipv6Group + `:)?` + ipv6Group + `)?::(?:` + ipv6Group + `:){3}` + ipv6Low32Bits +
		`|(?:(?:` + ipv6Group + `:){0,2}` + ipv6Group + `)?::(?:` + ipv6Group + `:){2}` + ipv6Low32Bits +
		`|(?:(?:` + ipv6Group + `:){0,3}` + ipv6Group + `)?::` + ipv6Group + `:` + ipv6Low32Bits +
		`|(?:(?:` + ipv6Group + `:){0,4}` + ipv6Group + `)?::` + ipv6Low32Bits +
		`|(?:(?:` + ipv6Group + `:){0,5}` + ipv6Group + `)?::` + ipv6Group +
		`|(?:(?:` + ipv6Group + `:){0,6}` + ipv6Group + `)?::` +
		`)`
	ipv6Zone       = `%.+`
	ipv4PrefixBits = `/(?:3[0-2]|[12]?[0-9])`
	ipv6PrefixBits = `/(?:12[0-8]|1[01][0-9]|[1-9]?[0-9])`
)

func IPAddressNode() *Node {
	return &Node{Type: "string", Pattern: `^(?:` + ipv4Address + `|` + ipv6Address + `(?:` + ipv6Zone + `)?)$`}
}

func IPPrefixNode() *Node {
	return &Node{Type: "string", Pattern: `^(?:` + ipv4Address + ipv4PrefixBits + `|` + ipv6Address + ipv6PrefixBits + `)$`}
}

func IPAddressOrPrefixNode() *Node {
	return &Node{Type: "string", Pattern: `^(?:` + ipv4Address + `(?:` + ipv4PrefixBits + `)?|` + ipv6Address + `(?:` + ipv6PrefixBits + `|` + ipv6Zone + `)?)$`}
}

func StringEnum(values ...string) *Node {
	anyValues := make([]any, 0, len(values))
	for _, value := range values {
		anyValues = append(anyValues, value)
	}
	return &Node{Type: "string", Enum: anyValues}
}

func StringConst(value string) *Node {
	return &Node{Const: value}
}

func AnyOf(nodes ...*Node) *Node {
	return &Node{AnyOf: nodes}
}

func OneOf(nodes ...*Node) *Node {
	return &Node{OneOf: nodes}
}

func ListableOf(element *Node) *Node {
	return AnyOf(element, &Node{Type: "array", Items: element})
}

func RefNode(name string) *Node {
	return &Node{Ref: "#/$defs/" + name}
}
