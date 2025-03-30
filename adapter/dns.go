package adapter

import (
	"context"
	"net/netip"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/service"

	"github.com/miekg/dns"
)

type DNSRouter interface {
	Lifecycle
	Exchange(ctx context.Context, message *dns.Msg, options DNSQueryOptions) (*dns.Msg, error)
	Lookup(ctx context.Context, domain string, options DNSQueryOptions) ([]netip.Addr, error)
	ClearCache()
	LookupReverseMapping(ip netip.Addr) (string, bool)
	ResetNetwork()
}

type DNSClient interface {
	Start()
	Exchange(ctx context.Context, transport DNSTransport, message *dns.Msg, options DNSQueryOptions, responseChecker func(responseAddrs []netip.Addr) bool) (*dns.Msg, error)
	Lookup(ctx context.Context, transport DNSTransport, domain string, options DNSQueryOptions, responseChecker func(responseAddrs []netip.Addr) bool) ([]netip.Addr, error)
	LookupCache(domain string, strategy C.DomainStrategy) ([]netip.Addr, bool)
	ExchangeCache(ctx context.Context, message *dns.Msg) (*dns.Msg, bool)
	ClearCache()
}

type DNSQueryOptions struct {
	Transport      DNSTransport
	Strategy       C.DomainStrategy
	LookupStrategy C.DomainStrategy
	DisableCache   bool
	RewriteTTL     *uint32
	ClientSubnet   netip.Prefix
}

func DNSQueryOptionsFrom(ctx context.Context, options *option.DomainResolveOptions) (*DNSQueryOptions, error) {
	if options == nil {
		return &DNSQueryOptions{}, nil
	}
	transportManager := service.FromContext[DNSTransportManager](ctx)
	transport, loaded := transportManager.Transport(options.Server)
	if !loaded {
		return nil, E.New("domain resolver not found: " + options.Server)
	}
	return &DNSQueryOptions{
		Transport:    transport,
		Strategy:     C.DomainStrategy(options.Strategy),
		DisableCache: options.DisableCache,
		RewriteTTL:   options.RewriteTTL,
		ClientSubnet: options.ClientSubnet.Build(netip.Prefix{}),
	}, nil
}

type RDRCStore interface {
	LoadRDRC(transportName string, qName string, qType uint16) (rejected bool)
	SaveRDRC(transportName string, qName string, qType uint16) error
	SaveRDRCAsync(transportName string, qName string, qType uint16, logger logger.Logger)
}

type DNSTransport interface {
	Lifecycle
	Type() string
	Tag() string
	Dependencies() []string
	Exchange(ctx context.Context, message *dns.Msg) (*dns.Msg, error)
}

type LegacyDNSTransport interface {
	LegacyStrategy() C.DomainStrategy
	LegacyClientSubnet() netip.Prefix
}

type DNSTransportRegistry interface {
	option.DNSTransportOptionsRegistry
	CreateDNSTransport(ctx context.Context, logger log.ContextLogger, tag string, transportType string, options any) (DNSTransport, error)
}

type DNSTransportManager interface {
	Lifecycle
	Transports() []DNSTransport
	Transport(tag string) (DNSTransport, bool)
	Default() DNSTransport
	FakeIP() FakeIPTransport
	Remove(tag string) error
	Create(ctx context.Context, logger log.ContextLogger, tag string, outboundType string, options any) error
}
