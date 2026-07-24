package option

import (
	"context"

	"github.com/sagernet/sing-box/schema"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/service"
)

type ServiceOptionsRegistry interface {
	OptionTypes() []string
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

func (h Service) DescribeSchema(builder schema.Builder) (*schema.Node, error) {
	return builder.Define("Service", func() (*schema.Node, error) {
		registry := service.FromContext[ServiceOptionsRegistry](builder.Context())
		if registry == nil {
			return nil, E.New("missing service options registry in context")
		}
		return registryUnion(builder, registry, nil, true)
	})
}
