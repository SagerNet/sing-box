package outbound

import (
	"context"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
)

type ConstructorFunc[T any] func(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options T) (adapter.Outbound, error)

func Register[Options any](registry *Registry, outboundType string, constructor ConstructorFunc[Options]) {
	registry.register(outboundType, func() any {
		return new(Options)
	}, func(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, rawOptions any) (adapter.Outbound, error) {
		var options *Options
		if rawOptions != nil {
			options = rawOptions.(*Options)
		}
		return constructor(ctx, router, logger, tag, common.PtrValueOrDefault(options))
	})
}

var _ adapter.OutboundRegistry = (*Registry)(nil)

type (
	optionsConstructorFunc func() any
	constructorFunc        func(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options any) (adapter.Outbound, error)
)

type Registry struct {
	access       sync.Mutex
	optionsType  map[string]optionsConstructorFunc
	constructors map[string]constructorFunc
}

func NewRegistry() *Registry {
	return &Registry{
		optionsType:  make(map[string]optionsConstructorFunc),
		constructors: make(map[string]constructorFunc),
	}
}

func (r *Registry) CreateOptions(outboundType string) (any, bool) {
	r.access.Lock()
	defer r.access.Unlock()
	optionsConstructor, loaded := r.optionsType[outboundType]
	if !loaded {
		return nil, false
	}
	return optionsConstructor(), true
}

func (r *Registry) CreateOutbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, outboundType string, options any) (adapter.Outbound, error) {
	r.access.Lock()
	defer r.access.Unlock()
	constructor, loaded := r.constructors[outboundType]
	if !loaded {
		return nil, E.New("outbound type not found: " + outboundType)
	}
	return constructor(ctx, router, logger, tag, options)
}

func (r *Registry) register(outboundType string, optionsConstructor optionsConstructorFunc, constructor constructorFunc) {
	r.access.Lock()
	defer r.access.Unlock()
	r.optionsType[outboundType] = optionsConstructor
	r.constructors[outboundType] = constructor
}
