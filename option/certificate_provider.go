package option

import (
	"context"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/service"
)

type CertificateProviderOptionsRegistry interface {
	CreateOptions(providerType string) (any, bool)
}

type _CertificateProvider struct {
	Type    string `json:"type"`
	Tag     string `json:"tag,omitempty"`
	Options any    `json:"-"`
}

type CertificateProvider _CertificateProvider

func (h *CertificateProvider) MarshalJSONContext(ctx context.Context) ([]byte, error) {
	return badjson.MarshallObjectsContext(ctx, (*_CertificateProvider)(h), h.Options)
}

func (h *CertificateProvider) UnmarshalJSONContext(ctx context.Context, content []byte) error {
	err := json.UnmarshalContext(ctx, content, (*_CertificateProvider)(h))
	if err != nil {
		return err
	}
	registry := service.FromContext[CertificateProviderOptionsRegistry](ctx)
	if registry == nil {
		return E.New("missing certificate provider options registry in context")
	}
	options, loaded := registry.CreateOptions(h.Type)
	if !loaded {
		return E.New("unknown certificate provider type: ", h.Type)
	}
	err = badjson.UnmarshallExcludedContext(ctx, content, (*_CertificateProvider)(h), options)
	if err != nil {
		return err
	}
	h.Options = options
	return nil
}

type CertificateProviderOptions struct {
	Tag     string `json:"-"`
	Type    string `json:"-"`
	Options any    `json:"-"`
}

type _CertificateProviderInline struct {
	Type string `json:"type"`
}

func (o *CertificateProviderOptions) MarshalJSONContext(ctx context.Context) ([]byte, error) {
	if o.Tag != "" {
		return json.Marshal(o.Tag)
	}
	return badjson.MarshallObjectsContext(ctx, _CertificateProviderInline{Type: o.Type}, o.Options)
}

func (o *CertificateProviderOptions) UnmarshalJSONContext(ctx context.Context, content []byte) error {
	if len(content) == 0 {
		return E.New("empty certificate_provider value")
	}
	if content[0] == '"' {
		return json.UnmarshalContext(ctx, content, &o.Tag)
	}
	var inline _CertificateProviderInline
	err := json.UnmarshalContext(ctx, content, &inline)
	if err != nil {
		return err
	}
	o.Type = inline.Type
	if o.Type == "" {
		return E.New("missing certificate provider type")
	}
	registry := service.FromContext[CertificateProviderOptionsRegistry](ctx)
	if registry == nil {
		return E.New("missing certificate provider options registry in context")
	}
	options, loaded := registry.CreateOptions(o.Type)
	if !loaded {
		return E.New("unknown certificate provider type: ", o.Type)
	}
	err = badjson.UnmarshallExcludedContext(ctx, content, &inline, options)
	if err != nil {
		return err
	}
	o.Options = options
	return nil
}

func (o *CertificateProviderOptions) IsShared() bool {
	return o.Tag != ""
}
