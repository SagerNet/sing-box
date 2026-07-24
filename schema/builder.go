package schema

import (
	"context"
	"reflect"

	E "github.com/sagernet/sing/common/exceptions"
)

type Builder interface {
	Context() context.Context
	Describe(valueType reflect.Type) (*Node, error)
	FlattenStruct(node *Node, structType reflect.Type) error
	Define(name string, build func() (*Node, error)) (*Node, error)
}

type Describer interface {
	DescribeSchema(builder Builder) (*Node, error)
}

type UnionVariant struct {
	Value        any
	StructType   reflect.Type
	TypeOptional bool
}

func DiscriminatedUnion(builder Builder, discriminatorKey string, discriminatorRequired bool, variants []UnionVariant, buildBase func(variant *Node) error) (*Node, error) {
	variantNodes := make([]*Node, 0, len(variants))
	for _, variant := range variants {
		variantNode := StrictObject()
		if variant.TypeOptional {
			stringValue, isString := variant.Value.(string)
			if !isString {
				return nil, E.New("optional discriminator requires a string value")
			}
			variantNode.Properties.Put(discriminatorKey, StringEnum(stringValue, ""))
		} else {
			variantNode.Properties.Put(discriminatorKey, &Node{Const: variant.Value})
		}
		if discriminatorRequired || !variant.TypeOptional {
			variantNode.Required = append(variantNode.Required, discriminatorKey)
		}
		if buildBase != nil {
			err := buildBase(variantNode)
			if err != nil {
				return nil, err
			}
		}
		if variant.StructType != nil {
			err := builder.FlattenStruct(variantNode, variant.StructType)
			if err != nil {
				return nil, err
			}
		}
		variantNodes = append(variantNodes, variantNode)
	}
	return OneOf(variantNodes...), nil
}
