package option

import (
	"context"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/service"
)

type EndpointOptionsRegistry interface {
	CreateOptions(endpointType string) (any, bool)
}

type _Endpoint struct {
	Type    string `json:"type"`
	Tag     string `json:"tag,omitempty"`
	Options any    `json:"-"`
}

type Endpoint _Endpoint

func (h *Endpoint) MarshalJSONContext(ctx context.Context) ([]byte, error) {
	return badjson.MarshallObjectsContext(ctx, (*_Endpoint)(h), h.Options)
}

func (h *Endpoint) UnmarshalJSONContext(ctx context.Context, content []byte) error {
	err := json.UnmarshalContext(ctx, content, (*_Endpoint)(h))
	if err != nil {
		return err
	}
	registry := service.FromContext[EndpointOptionsRegistry](ctx)
	if registry == nil {
		return E.New("missing endpoint fields registry in context")
	}
	options, loaded := registry.CreateOptions(h.Type)
	if !loaded {
		return E.New("unknown endpoint type: ", h.Type)
	}
	err = badjson.UnmarshallExcludedContext(ctx, content, (*_Endpoint)(h), options)
	if err != nil {
		return err
	}
	h.Options = options
	return nil
}
