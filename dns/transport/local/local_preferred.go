package local

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/dns/transport/hosts"
	"github.com/sagernet/sing-box/dns/transport/mdns"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/service"

	mDNS "github.com/miekg/dns"
)

type PreferredDomainResolver struct {
	ctx              context.Context
	logger           logger.ContextLogger
	hosts            *hosts.File
	neighborResolver adapter.NeighborResolver
	neighborSuffixes []string
}

func NewPreferredDomainResolver(ctx context.Context, contextLogger logger.ContextLogger, options option.LocalDNSServerOptions) (*PreferredDomainResolver, error) {
	suffixes, err := buildNeighborMatchers(options.NeighborDomain)
	if err != nil {
		return nil, err
	}
	return &PreferredDomainResolver{
		ctx:              ctx,
		logger:           contextLogger,
		neighborSuffixes: suffixes,
	}, nil
}

func (r *PreferredDomainResolver) Start(stage adapter.StartStage) {
	switch stage {
	case adapter.StartStateInitialize:
		defaultHosts, err := hosts.NewDefault()
		if err != nil {
			r.logger.Warn(err)
		} else {
			r.hosts = defaultHosts
		}
	case adapter.StartStateStart:
		router := service.FromContext[adapter.Router](r.ctx)
		if router != nil {
			r.neighborResolver = router.NeighborResolver()
		}
	}
}

func (r *PreferredDomainResolver) PreferredDomain(domain string) bool {
	if r.hosts != nil {
		if len(r.hosts.Lookup(dns.FqdnToDomain(domain))) > 0 {
			return true
		}
	}
	return r.hasNeighborHost(domain) || mdns.IsLocalDomain(domain)
}

func (r *PreferredDomainResolver) Lookup(message *mDNS.Msg) *mDNS.Msg {
	question := message.Question[0]
	if question.Qtype != mDNS.TypeA && question.Qtype != mDNS.TypeAAAA {
		return nil
	}
	if r.hosts != nil {
		addresses := r.hosts.Lookup(dns.FqdnToDomain(question.Name))
		if len(addresses) > 0 {
			return dns.FixedResponse(message.Id, question, addresses, C.DefaultDNSTTL)
		}
	}
	return r.lookupNeighbor(message)
}
