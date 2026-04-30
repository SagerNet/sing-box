package local

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/dns/transport/hosts"
	"github.com/sagernet/sing-box/dns/transport/mdns"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"

	mDNS "github.com/miekg/dns"
)

func RegisterTransport(registry *dns.TransportRegistry) {
	dns.RegisterTransport[option.LocalDNSServerOptions](registry, C.DNSTypeLocal, NewTransport)
}

var (
	_ adapter.DNSTransport                    = (*Transport)(nil)
	_ adapter.DNSTransportWithPreferredDomain = (*Transport)(nil)
)

type Transport struct {
	dns.TransportAdapter
	ctx              context.Context
	logger           logger.ContextLogger
	hosts            *hosts.File
	dialer           N.Dialer
	preferGo         bool
	fallback         bool
	resolved         ResolvedResolver
	mdnsTransport    adapter.DNSTransport
	dhcpTransport    dhcpTransport
	neighborResolver adapter.NeighborResolver
	neighborSuffixes []string
}

type dhcpTransport interface {
	adapter.DNSTransport
	Fetch() []M.Socksaddr
	Exchange0(ctx context.Context, message *mDNS.Msg, servers []M.Socksaddr) (*mDNS.Msg, error)
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
		if !t.preferGo && isSystemdResolvedManaged() {
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
	case adapter.StartStateStart:
		if C.IsDarwin {
			inboundManager := service.FromContext[adapter.InboundManager](t.ctx)
			for _, inbound := range inboundManager.Inbounds() {
				if inbound.Type() == C.TypeTun {
					t.fallback = true
					break
				}
			}
			if t.fallback {
				t.dhcpTransport = newDHCPTransport(t.TransportAdapter, log.ContextWithOverrideLevel(t.ctx, log.LevelDebug), t.dialer, t.logger)
			}
		} else {
			t.mdnsTransport = mdns.NewRawTransport(t.TransportAdapter, t.ctx, t.logger)
		}
		router := service.FromContext[adapter.Router](t.ctx)
		if router != nil {
			t.neighborResolver = router.NeighborResolver()
		}
		fallthrough
	default:
		if t.dhcpTransport != nil {
			err := t.dhcpTransport.Start(stage)
			if err != nil {
				return err
			}
		}
		if t.mdnsTransport != nil {
			err := t.mdnsTransport.Start(stage)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *Transport) Close() error {
	return common.Close(t.resolved, t.dhcpTransport, t.mdnsTransport)
}

func (t *Transport) Reset() {
	if t.dhcpTransport != nil {
		t.dhcpTransport.Reset()
	}
	if t.mdnsTransport != nil {
		t.mdnsTransport.Reset()
	}
}

func (t *Transport) PreferredDomain(domain string) bool {
	if t.hosts != nil {
		if len(t.hosts.Lookup(dns.FqdnToDomain(domain))) > 0 {
			return true
		}
	}
	return t.hasNeighborHost(domain) || mdns.IsLocalDomain(domain)
}

func (t *Transport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
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
	if mdns.IsLocalDomain(question.Name) {
		if C.IsDarwin {
			return t.systemExchange(ctx, message)
		}
		return t.mdnsTransport.Exchange(ctx, message)
	}
	if t.resolved != nil {
		return t.resolved.Exchange(ctx, message)
	}
	if t.dhcpTransport != nil {
		servers := t.dhcpTransport.Fetch()
		if len(servers) > 0 {
			return t.dhcpTransport.Exchange0(ctx, message, servers)
		}
	}
	if t.fallback {
		return t.systemExchange(ctx, message)
	}
	return t.exchange(ctx, message, question.Name)
}
