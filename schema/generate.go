package schema

import (
	"context"
	stdjson "encoding/json"
	"reflect"
)

func Generate(ctx context.Context, rootType reflect.Type) ([]byte, error) {
	g := &generator{
		ctx:      ctx,
		defs:     make(map[string]*Node),
		defTypes: make(map[reflect.Type]string),
	}
	root, err := g.Describe(rootType)
	if err != nil {
		return nil, err
	}
	root.Defs = g.sortedDefs()
	content, err := stdjson.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}
