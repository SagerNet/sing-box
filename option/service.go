package option

import (
	"context"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/service"
)

type ServiceOptionsRegistry interface {
	CreateOptions(serviceType string) (any, bool)
}

type _Service struct {
	Type    string `json:"type"`
	Tag     string `json:"tag,omitempty"`
	Options any    `json:"-"`
}

type Service _Service

func (h *Service) MarshalJSONContext(ctx context.Context) ([]byte, error) {
	return badjson.MarshallObjectsContext(ctx, (*_Service)(h), h.Options)
}

func (h *Service) UnmarshalJSONContext(ctx context.Context, content []byte) error {
	err := json.UnmarshalContext(ctx, content, (*_Service)(h))
	if err != nil {
		return err
	}
	registry := service.FromContext[ServiceOptionsRegistry](ctx)
	if registry == nil {
		return E.New("missing service fields registry in context")
	}
	options, loaded := registry.CreateOptions(h.Type)
	if !loaded {
		return E.New("unknown inbound type: ", h.Type)
	}
	err = badjson.UnmarshallExcludedContext(ctx, content, (*_Service)(h), options)
	if err != nil {
		return err
	}
	h.Options = options
	return nil
}
