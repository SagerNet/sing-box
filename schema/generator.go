package schema

import (
	"context"
	"encoding"
	stdjson "encoding/json"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/sagernet/sing/common/byteformats"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
)

var (
	jsonUnmarshalerType    = reflect.TypeFor[stdjson.Unmarshaler]()
	contextUnmarshalerType = reflect.TypeFor[json.ContextUnmarshaler]()
	textUnmarshalerType    = reflect.TypeFor[encoding.TextUnmarshaler]()

	durationType           = reflect.TypeFor[badoption.Duration]()
	addrType               = reflect.TypeFor[badoption.Addr]()
	prefixType             = reflect.TypeFor[badoption.Prefix]()
	prefixableType         = reflect.TypeFor[badoption.Prefixable]()
	httpHeaderType         = reflect.TypeFor[badoption.HTTPHeader]()
	memoryBytesType        = reflect.TypeFor[byteformats.MemoryBytes]()
	networkBytesCompatType = reflect.TypeFor[byteformats.NetworkBytesCompat]()
)

type generator struct {
	ctx      context.Context
	defs     map[string]*Node
	defTypes map[reflect.Type]string
	path     []string
}

func (g *generator) Context() context.Context {
	return g.ctx
}

func (g *generator) Define(name string, build func() (*Node, error)) (*Node, error) {
	_, exists := g.defs[name]
	if exists {
		return RefNode(name), nil
	}
	g.defs[name] = nil
	node, err := build()
	if err != nil {
		return nil, err
	}
	g.defs[name] = node
	return RefNode(name), nil
}

func implementationOf[T any](valueType reflect.Type) (T, bool) {
	interfaceType := reflect.TypeFor[T]()
	if !valueType.Implements(interfaceType) && !reflect.PointerTo(valueType).Implements(interfaceType) {
		var zeroValue T
		return zeroValue, false
	}
	return reflect.New(valueType).Interface().(T), true
}

func (g *generator) Describe(valueType reflect.Type) (*Node, error) {
	for valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	describer, described := implementationOf[Describer](valueType)
	if described {
		return describer.DescribeSchema(g)
	}
	switch valueType {
	case durationType:
		return g.Define("Duration", func() (*Node, error) {
			return DurationNode(), nil
		})
	case addrType, prefixType, prefixableType:
		return StringNode(), nil
	case httpHeaderType:
		return g.Define("HTTPHeader", func() (*Node, error) {
			return &Node{Type: "object", AdditionalProperties: ListableOf(StringNode())}, nil
		})
	case memoryBytesType, networkBytesCompatType:
		return AnyOf(UnsignedNode(64), StringNode()), nil
	}
	if isListable(valueType) {
		elementNode, err := g.Describe(valueType.Elem())
		if err != nil {
			return nil, err
		}
		return ListableOf(elementNode), nil
	}
	if isTypedMap(valueType) {
		return g.typedMapNode(valueType)
	}
	pointerType := reflect.PointerTo(valueType)
	if pointerType.Implements(jsonUnmarshalerType) || pointerType.Implements(contextUnmarshalerType) {
		return nil, E.New("unmapped custom JSON type ", valueType.String(), " at ", strings.Join(g.path, "."))
	}
	if pointerType.Implements(textUnmarshalerType) {
		return StringNode(), nil
	}
	switch valueType.Kind() {
	case reflect.Struct:
		if valueType.Name() == "" {
			node := StrictObject()
			err := g.FlattenStruct(node, valueType)
			if err != nil {
				return nil, err
			}
			return node, nil
		}
		return g.Define(g.defNameFor(valueType), func() (*Node, error) {
			node := StrictObject()
			err := g.FlattenStruct(node, valueType)
			if err != nil {
				return nil, err
			}
			return node, nil
		})
	case reflect.Slice, reflect.Array:
		if valueType.Kind() == reflect.Slice && valueType.Elem().Kind() == reflect.Uint8 {
			// encoding/json accepts both base64 strings and number arrays.
			return AnyOf(StringNode(), &Node{Type: "array", Items: UnsignedNode(8)}), nil
		}
		elementNode, err := g.Describe(valueType.Elem())
		if err != nil {
			return nil, err
		}
		return &Node{Type: "array", Items: elementNode}, nil
	case reflect.Map:
		if valueType.Key().Kind() != reflect.String {
			return nil, E.New("unsupported map key type ", valueType.String(), " at ", strings.Join(g.path, "."))
		}
		valueNode, err := g.Describe(valueType.Elem())
		if err != nil {
			return nil, err
		}
		return &Node{Type: "object", AdditionalProperties: valueNode}, nil
	case reflect.Bool:
		return BooleanNode(), nil
	case reflect.String:
		return StringNode(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return IntegerNode(), nil
	case reflect.Uint, reflect.Uint64:
		return UnsignedNode(64), nil
	case reflect.Uint8:
		return UnsignedNode(8), nil
	case reflect.Uint16:
		return UnsignedNode(16), nil
	case reflect.Uint32:
		return UnsignedNode(32), nil
	case reflect.Float32, reflect.Float64:
		return &Node{Type: "number"}, nil
	default:
		return nil, E.New("unsupported kind ", valueType.Kind().String(), " for ", valueType.String(), " at ", strings.Join(g.path, "."))
	}
}

func (g *generator) defNameFor(fieldType reflect.Type) string {
	existingName, loaded := g.defTypes[fieldType]
	if loaded {
		return existingName
	}
	name := fieldType.Name()
	for otherType, otherName := range g.defTypes {
		if otherName == name && otherType != fieldType {
			name = pathBase(fieldType.PkgPath()) + "." + name
			break
		}
	}
	g.defTypes[fieldType] = name
	return name
}

func pathBase(packagePath string) string {
	index := strings.LastIndexByte(packagePath, '/')
	if index < 0 {
		return packagePath
	}
	return packagePath[index+1:]
}

// FlattenStruct merges the JSON fields of structType into node, following the
// same flattening semantics as badjson.MarshallObjects / anonymous embedding.
func (g *generator) FlattenStruct(node *Node, structType reflect.Type) error {
	for structType.Kind() == reflect.Pointer {
		structType = structType.Elem()
	}
	if structType.Kind() != reflect.Struct {
		return E.New("cannot flatten non-struct type ", structType.String(), " at ", strings.Join(g.path, "."))
	}
	for i := range structType.NumField() {
		field := structType.Field(i)
		if !field.IsExported() && !field.Anonymous {
			continue
		}
		tagValue := field.Tag.Get("json")
		tagName, _, _ := strings.Cut(tagValue, ",")
		if tagName == "-" {
			continue
		}
		fieldType := field.Type
		for fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		if field.Tag.Get("schema") == "omit" {
			continue
		}
		if field.Anonymous && tagName == "" {
			err := g.FlattenStruct(node, fieldType)
			if err != nil {
				return err
			}
			continue
		}
		if tagName == "" {
			tagName = field.Name
		}
		enumTag := field.Tag.Get("enum")
		examplesTag := field.Tag.Get("examples")
		referenceTag := field.Tag.Get("reference")
		g.path = append(g.path, structType.Name()+"."+tagName)
		var fieldNode *Node
		var err error
		if enumTag != "" || examplesTag != "" || referenceTag != "" {
			fieldNode, err = taggedFieldNode(fieldType, enumTag, examplesTag, referenceTag)
		} else {
			fieldNode, err = g.Describe(fieldType)
		}
		g.path = g.path[:len(g.path)-1]
		if err != nil {
			return err
		}
		node.Properties.Put(tagName, fieldNode)
	}
	return nil
}

func taggedFieldNode(fieldType reflect.Type, enumTag string, examplesTag string, referenceTag string) (*Node, error) {
	elementType := fieldType
	for elementType.Kind() == reflect.Pointer {
		elementType = elementType.Elem()
	}
	listable := isListable(fieldType)
	plainSlice := !listable && elementType.Kind() == reflect.Slice && elementType.Elem().Kind() == reflect.String
	if listable {
		elementType = fieldType.Elem()
	} else if plainSlice {
		elementType = elementType.Elem()
	}
	var element *Node
	var err error
	if enumTag != "" {
		element, err = taggedValueNode(elementType, strings.Split(enumTag, ","))
		if err != nil {
			return nil, err
		}
	} else {
		element, err = taggedValueNode(elementType, nil)
		if err != nil {
			return nil, err
		}
	}
	if examplesTag != "" {
		examples, parseErr := taggedValues(elementType, strings.Split(examplesTag, ","))
		if parseErr != nil {
			return nil, parseErr
		}
		element.Examples = examples
	}
	if referenceTag != "" {
		if elementType.Kind() != reflect.String {
			return nil, E.New("reference tags require a string field, got ", fieldType.String())
		}
		element.TagReference = referenceTag
	}
	if listable {
		return ListableOf(element), nil
	}
	if plainSlice {
		return &Node{Type: "array", Items: element}, nil
	}
	return element, nil
}

func taggedValueNode(fieldType reflect.Type, values []string) (*Node, error) {
	switch fieldType.Kind() {
	case reflect.String:
		node := StringNode()
		for _, value := range values {
			node.Enum = append(node.Enum, value)
		}
		return node, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		node := IntegerNode()
		enumValues, err := taggedValues(fieldType, values)
		if err != nil {
			return nil, err
		}
		node.Enum = enumValues
		return node, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		node := UnsignedNode(fieldType.Bits())
		enumValues, err := taggedValues(fieldType, values)
		if err != nil {
			return nil, err
		}
		node.Enum = enumValues
		return node, nil
	default:
		node := StringNode()
		for _, value := range values {
			err := unmarshalTaggedValue(fieldType, value)
			if err != nil {
				return nil, err
			}
			node.Enum = append(node.Enum, value)
		}
		return node, nil
	}
}

func unmarshalTaggedValue(fieldType reflect.Type, value string) error {
	err := json.Unmarshal([]byte(strconv.Quote(value)), reflect.New(fieldType).Interface())
	if err != nil {
		return E.Cause(err, "unmarshal tagged value ", value, " as ", fieldType.String())
	}
	return nil
}

func taggedValues(fieldType reflect.Type, values []string) ([]any, error) {
	result := make([]any, 0, len(values))
	for _, value := range values {
		switch fieldType.Kind() {
		case reflect.String:
			result = append(result, value)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			integerValue, err := strconv.ParseInt(value, 10, fieldType.Bits())
			if err != nil {
				return nil, E.Cause(err, "parse enum value ", value, " for ", fieldType.String())
			}
			result = append(result, integerValue)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			unsignedValue, err := strconv.ParseUint(value, 10, fieldType.Bits())
			if err != nil {
				return nil, E.Cause(err, "parse enum value ", value, " for ", fieldType.String())
			}
			result = append(result, unsignedValue)
		default:
			err := unmarshalTaggedValue(fieldType, value)
			if err != nil {
				return nil, err
			}
			result = append(result, value)
		}
	}
	return result, nil
}

func isListable(fieldType reflect.Type) bool {
	return fieldType.Kind() == reflect.Slice &&
		fieldType.PkgPath() == "github.com/sagernet/sing/common/json/badoption" &&
		strings.HasPrefix(fieldType.Name(), "Listable[")
}

func isTypedMap(fieldType reflect.Type) bool {
	return fieldType.Kind() == reflect.Struct &&
		fieldType.PkgPath() == "github.com/sagernet/sing/common/json/badjson" &&
		strings.HasPrefix(fieldType.Name(), "TypedMap[")
}

func (g *generator) typedMapNode(fieldType reflect.Type) (*Node, error) {
	mapField, found := fieldType.FieldByName("Map")
	if !found {
		return nil, E.New("unexpected TypedMap layout: missing Map in ", fieldType.String())
	}
	rawMapField, found := mapField.Type.FieldByName("rawMap")
	if !found {
		return nil, E.New("unexpected TypedMap layout: missing rawMap in ", fieldType.String())
	}
	keyType := rawMapField.Type.Key()
	elementValueField, found := rawMapField.Type.Elem().Elem().FieldByName("Value")
	if !found {
		return nil, E.New("unexpected TypedMap layout: missing element value in ", fieldType.String())
	}
	entryValueField, found := elementValueField.Type.FieldByName("Value")
	if !found {
		return nil, E.New("unexpected TypedMap layout: missing entry value in ", fieldType.String())
	}
	valueNode, err := g.Describe(entryValueField.Type)
	if err != nil {
		return nil, err
	}
	node := &Node{Type: "object", AdditionalProperties: valueNode}
	if keyType.Kind() != reflect.String || keyType.PkgPath() != "" {
		keyNode, keyErr := g.Describe(keyType)
		if keyErr != nil {
			return nil, keyErr
		}
		node.PropertyNames = keyNode
	}
	return node, nil
}

func (g *generator) sortedDefs() *badjson.TypedMap[string, *Node] {
	names := make([]string, 0, len(g.defs))
	for name := range g.defs {
		names = append(names, name)
	}
	slices.Sort(names)
	result := new(badjson.TypedMap[string, *Node])
	for _, name := range names {
		result.Put(name, g.defs[name])
	}
	return result
}
