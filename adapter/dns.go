package adapter

import (
	"context"
	"net/netip"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/logger"

	"github.com/miekg/dns"
)

type DNSRouter interface {
	Lifecycle
	Exchange(ctx context.Context, message *dns.Msg, options DNSQueryOptions) (*dns.Msg, error)
	ExchangeAsync(ctx context.Context, message *dns.Msg, options DNSQueryOptions, callback func(response *dns.Msg, err error))
	Lookup(ctx context.Context, domain string, options DNSQueryOptions) ([]netip.Addr, error)
	ClearCache()
	LookupReverseMapping(ip netip.Addr) (string, bool)
	ResetNetwork()
}

type DNSClient interface {
	Start()
	Exchange(ctx context.Context, transport DNSTransport, message *dns.Msg, options DNSQueryOptions, responseChecker func(response *dns.Msg) bool) (*dns.Msg, error)
	ExchangeAsync(ctx context.Context, transport DNSTransport, message *dns.Msg, options DNSQueryOptions, responseChecker func(response *dns.Msg) bool, callback func(response *dns.Msg, err error))
	Lookup(ctx context.Context, transport DNSTransport, domain string, options DNSQueryOptions, responseChecker func(response *dns.Msg) bool) ([]netip.Addr, error)
	ClearCache()
}

type DNSQueryOptions struct {
	Transport              DNSTransport
	Strategy               C.DomainStrategy
	LookupStrategy         C.DomainStrategy
	DisableCache           bool
	DisableOptimisticCache bool
	RewriteTTL             *uint32
	Timeout                time.Duration
	ClientSubnet           netip.Prefix
	RemoveClientSubnet     bool
}

type RDRCStore interface {
	LoadRDRC(transportName string, qName string, qType uint16) (rejected bool)
	SaveRDRC(transportName string, qName string, qType uint16) error
	SaveRDRCAsync(transportName string, qName string, qType uint16, logger logger.Logger)
}

type DNSCacheStore interface {
	LoadDNSCache(transportName string, qName string, qType uint16) (rawMessage []byte, expireAt time.Time, loaded bool)
	SaveDNSCache(transportName string, qName string, qType uint16, rawMessage []byte, expireAt time.Time) error
	SaveDNSCacheAsync(transportName string, qName string, qType uint16, rawMessage []byte, expireAt time.Time, logger logger.Logger)
	ClearDNSCache() error
}

type DNSTransport interface {
	Lifecycle
	Type() string
	Tag() string
	Dependencies() []string
	// Reset closes the transport's existing connections so later requests use fresh connections.
	// Exchanges that are currently using those connections may fail.
	Reset()
	Exchange(ctx context.Context, message *dns.Msg) (*dns.Msg, error)
	ExchangeAsync(ctx context.Context, message *dns.Msg, callback func(response *dns.Msg, err error))
}

type DNSTransportWithPreferredDomain interface {
	DNSTransport
	PreferredDomain(domain string) bool
}

type DNSTransportWithEnvironment interface {
	DNSTransport
	Environment() []string
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
