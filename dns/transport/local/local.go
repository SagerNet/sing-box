//go:build !darwin

package local

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/dns/transport/hosts"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"

	mDNS "github.com/miekg/dns"
)

func RegisterTransport(registry *dns.TransportRegistry) {
	dns.RegisterTransport[option.LocalDNSServerOptions](registry, C.DNSTypeLocal, NewTransport)
}

var _ adapter.DNSTransport = (*Transport)(nil)

type Transport struct {
	dns.TransportAdapter
	ctx              context.Context
	logger           logger.ContextLogger
	hosts            *hosts.File
	dialer           N.Dialer
	preferGo         bool
	resolved         ResolvedResolver
	neighborResolver adapter.NeighborResolver
	neighborSuffixes []string
}

func NewTransport(ctx context.Context, logger log.ContextLogger, tag string, options option.LocalDNSServerOptions) (adapter.DNSTransport, error) {
	transportDialer, err := dns.NewLocalDialer(ctx, options)
	if err != nil {
		return nil, err
	}
	suffixes, err := buildNeighborMatchers(options.NeighborDomain)
	if err != nil {
		return nil, err
	}
	return &Transport{
		TransportAdapter: dns.NewTransportAdapterWithLocalOptions(C.DNSTypeLocal, tag, options),
		ctx:              ctx,
		logger:           logger,
		dialer:           transportDialer,
		preferGo:         options.PreferGo,
		neighborSuffixes: suffixes,
	}, nil
}

func (t *Transport) Start(stage adapter.StartStage) error {
	switch stage {
	case adapter.StartStateInitialize:
		defaultHosts, err := hosts.NewDefault()
		if err != nil {
			t.logger.Warn(err)
		} else {
			t.hosts = defaultHosts
		}
		if !t.preferGo {
			if isSystemdResolvedManaged() {
				resolvedResolver, err := NewResolvedResolver(t.ctx, t.logger)
				if err == nil {
					err = resolvedResolver.Start()
					if err == nil {
						t.resolved = resolvedResolver
					} else {
						t.logger.Warn(E.Cause(err, "initialize resolved resolver"))
					}
				}
			}
		}
	case adapter.StartStateStart:
		router := service.FromContext[adapter.Router](t.ctx)
		if router != nil {
			t.neighborResolver = router.NeighborResolver()
		}
	}
	return nil
}

func (t *Transport) Close() error {
	if t.resolved != nil {
		return t.resolved.Close()
	}
	return nil
}

func (t *Transport) Reset() {
}

func (t *Transport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	if t.resolved != nil {
		response := t.lookupNeighbor(message)
		if response != nil {
			return response, nil
		}
		return t.resolved.Exchange(ctx, message)
	}
	question := message.Question[0]
	if t.hosts != nil && (question.Qtype == mDNS.TypeA || question.Qtype == mDNS.TypeAAAA) {
		addresses := t.hosts.Lookup(dns.FqdnToDomain(question.Name))
		if len(addresses) > 0 {
			return dns.FixedResponse(message.Id, question, addresses, C.DefaultDNSTTL), nil
		}
	}
	response := t.lookupNeighbor(message)
	if response != nil {
		return response, nil
	}
	return t.exchange(ctx, message, question.Name)
}
