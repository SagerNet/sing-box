package inbound

import (
	"context"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
)

type ConstructorFunc[T any] func(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options T) (adapter.Inbound, error)

func Register[Options any](registry *Registry, outboundType string, constructor ConstructorFunc[Options]) {
	registry.register(outboundType, func() any {
		return new(Options)
	}, func(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, rawOptions any) (adapter.Inbound, error) {
		var options *Options
		if rawOptions != nil {
			options = rawOptions.(*Options)
		}
		return constructor(ctx, router, logger, tag, common.PtrValueOrDefault(options))
	})
}

var _ adapter.InboundRegistry = (*Registry)(nil)

type (
	optionsConstructorFunc func() any
	constructorFunc        func(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options any) (adapter.Inbound, error)
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

func (m *Registry) CreateOptions(outboundType string) (any, bool) {
	m.access.Lock()
	defer m.access.Unlock()
	optionsConstructor, loaded := m.optionsType[outboundType]
	if !loaded {
		return nil, false
	}
	return optionsConstructor(), true
}

func (m *Registry) Create(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, outboundType string, options any) (adapter.Inbound, error) {
	m.access.Lock()
	defer m.access.Unlock()
	constructor, loaded := m.constructor[outboundType]
	if !loaded {
		return nil, E.New("outbound type not found: " + outboundType)
	}
	return constructor(ctx, router, logger, tag, options)
}

func (m *Registry) register(outboundType string, optionsConstructor optionsConstructorFunc, constructor constructorFunc) {
	m.access.Lock()
	defer m.access.Unlock()
	m.optionsType[outboundType] = optionsConstructor
	m.constructor[outboundType] = constructor
}
