package certificate

import (
	"context"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
)

type ConstructorFunc[T any] func(ctx context.Context, logger log.ContextLogger, tag string, options T) (adapter.CertificateProviderService, error)

func Register[Options any](registry *Registry, providerType string, constructor ConstructorFunc[Options]) {
	registry.register(providerType, func() any {
		return new(Options)
	}, func(ctx context.Context, logger log.ContextLogger, tag string, rawOptions any) (adapter.CertificateProviderService, error) {
		var options *Options
		if rawOptions != nil {
			options = rawOptions.(*Options)
		}
		return constructor(ctx, logger, tag, common.PtrValueOrDefault(options))
	})
}

var _ adapter.CertificateProviderRegistry = (*Registry)(nil)

type (
	optionsConstructorFunc func() any
	constructorFunc        func(ctx context.Context, logger log.ContextLogger, tag string, options any) (adapter.CertificateProviderService, error)
)

type Registry struct {
	access      sync.Mutex
	optionsType map[string]optionsConstructorFunc
	constructor map[string]constructorFunc
}

func NewRegistry() *Registry {
	return &Registry{
		optionsType: make(map[string]optionsConstructorFunc),
		constructor: make(map[string]constructorFunc),
	}
}

func (m *Registry) CreateOptions(providerType string) (any, bool) {
	m.access.Lock()
	defer m.access.Unlock()
	optionsConstructor, loaded := m.optionsType[providerType]
	if !loaded {
		return nil, false
	}
	return optionsConstructor(), true
}

func (m *Registry) Create(ctx context.Context, logger log.ContextLogger, tag string, providerType string, options any) (adapter.CertificateProviderService, error) {
	m.access.Lock()
	defer m.access.Unlock()
	constructor, loaded := m.constructor[providerType]
	if !loaded {
		return nil, E.New("certificate provider type not found: " + providerType)
	}
	return constructor(ctx, logger, tag, options)
}

func (m *Registry) register(providerType string, optionsConstructor optionsConstructorFunc, constructor constructorFunc) {
	m.access.Lock()
	defer m.access.Unlock()
	m.optionsType[providerType] = optionsConstructor
	m.constructor[providerType] = constructor
}
