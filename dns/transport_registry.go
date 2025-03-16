package dns

import (
	"context"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
)

type TransportConstructorFunc[T any] func(ctx context.Context, logger log.ContextLogger, tag string, options T) (adapter.DNSTransport, error)

func RegisterTransport[Options any](registry *TransportRegistry, transportType string, constructor TransportConstructorFunc[Options]) {
	registry.register(transportType, func() any {
		return new(Options)
	}, func(ctx context.Context, logger log.ContextLogger, tag string, rawOptions any) (adapter.DNSTransport, error) {
		var options *Options
		if rawOptions != nil {
			options = rawOptions.(*Options)
		}
		return constructor(ctx, logger, tag, common.PtrValueOrDefault(options))
	})
}

var _ adapter.DNSTransportRegistry = (*TransportRegistry)(nil)

type (
	optionsConstructorFunc func() any
	constructorFunc        func(ctx context.Context, logger log.ContextLogger, tag string, options any) (adapter.DNSTransport, error)
)

type TransportRegistry struct {
	access       sync.Mutex
	optionsType  map[string]optionsConstructorFunc
	constructors map[string]constructorFunc
}

func NewTransportRegistry() *TransportRegistry {
	return &TransportRegistry{
		optionsType:  make(map[string]optionsConstructorFunc),
		constructors: make(map[string]constructorFunc),
	}
}

func (r *TransportRegistry) CreateOptions(transportType string) (any, bool) {
	r.access.Lock()
	defer r.access.Unlock()
	optionsConstructor, loaded := r.optionsType[transportType]
	if !loaded {
		return nil, false
	}
	return optionsConstructor(), true
}

func (r *TransportRegistry) CreateDNSTransport(ctx context.Context, logger log.ContextLogger, tag string, transportType string, options any) (adapter.DNSTransport, error) {
	r.access.Lock()
	defer r.access.Unlock()
	constructor, loaded := r.constructors[transportType]
	if !loaded {
		return nil, E.New("transport type not found: " + transportType)
	}
	return constructor(ctx, logger, tag, options)
}

func (r *TransportRegistry) register(transportType string, optionsConstructor optionsConstructorFunc, constructor constructorFunc) {
	r.access.Lock()
	defer r.access.Unlock()
	r.optionsType[transportType] = optionsConstructor
	r.constructors[transportType] = constructor
}
