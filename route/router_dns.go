package route

import (
	"context"
	"net/netip"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing/common/cache"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"

	mDNS "github.com/miekg/dns"
)

type DNSReverseMapping struct {
	cache *cache.LruCache[netip.Addr, string]
}

func NewDNSReverseMapping() *DNSReverseMapping {
	return &DNSReverseMapping{
		cache: cache.New[netip.Addr, string](),
	}
}

func (m *DNSReverseMapping) Save(address netip.Addr, domain string, ttl int) {
	m.cache.StoreWithExpire(address, domain, time.Now().Add(time.Duration(ttl)*time.Second))
}

func (m *DNSReverseMapping) Query(address netip.Addr) (string, bool) {
	domain, loaded := m.cache.Load(address)
	return domain, loaded
}

func (r *Router) matchDNS(ctx context.Context) (context.Context, dns.Transport, dns.DomainStrategy) {
	metadata := adapter.ContextFrom(ctx)
	if metadata == nil {
		panic("no context")
	}
	for i, rule := range r.dnsRules {
		if rule.Match(metadata) {
			if rule.DisableCache() {
				ctx = dns.ContextWithDisableCache(ctx, true)
			}
			detour := rule.Outbound()
			r.dnsLogger.DebugContext(ctx, "match[", i, "] ", rule.String(), " => ", detour)
			if transport, loaded := r.transportMap[detour]; loaded {
				if domainStrategy, dsLoaded := r.transportDomainStrategy[transport]; dsLoaded {
					return ctx, transport, domainStrategy
				} else {
					return ctx, transport, r.defaultDomainStrategy
				}
			}
			r.dnsLogger.ErrorContext(ctx, "transport not found: ", detour)
		}
	}
	if domainStrategy, dsLoaded := r.transportDomainStrategy[r.defaultTransport]; dsLoaded {
		return ctx, r.defaultTransport, domainStrategy
	} else {
		return ctx, r.defaultTransport, r.defaultDomainStrategy
	}
}

func (r *Router) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	if len(message.Question) > 0 {
		r.dnsLogger.DebugContext(ctx, "exchange ", formatQuestion(message.Question[0].String()))
	}
	ctx, metadata := adapter.AppendContext(ctx)
	if len(message.Question) > 0 {
		metadata.QueryType = message.Question[0].Qtype
		switch metadata.QueryType {
		case mDNS.TypeA:
			metadata.IPVersion = 4
		case mDNS.TypeAAAA:
			metadata.IPVersion = 6
		}
		metadata.Domain = fqdnToDomain(message.Question[0].Name)
	}
	ctx, transport, strategy := r.matchDNS(ctx)
	ctx, cancel := context.WithTimeout(ctx, C.DNSTimeout)
	defer cancel()
	response, err := r.dnsClient.Exchange(ctx, transport, message, strategy)
	if err != nil && len(message.Question) > 0 {
		r.dnsLogger.ErrorContext(ctx, E.Cause(err, "exchange failed for ", formatQuestion(message.Question[0].String())))
	}
	if len(message.Question) > 0 && response != nil {
		LogDNSAnswers(r.dnsLogger, ctx, message.Question[0].Name, response.Answer)
	}
	if r.dnsReverseMapping != nil && len(message.Question) > 0 && response != nil && len(response.Answer) > 0 {
		for _, answer := range response.Answer {
			switch record := answer.(type) {
			case *mDNS.A:
				r.dnsReverseMapping.Save(M.AddrFromIP(record.A), fqdnToDomain(record.Hdr.Name), int(record.Hdr.Ttl))
			case *mDNS.AAAA:
				r.dnsReverseMapping.Save(M.AddrFromIP(record.AAAA), fqdnToDomain(record.Hdr.Name), int(record.Hdr.Ttl))
			}
		}
	}
	return response, err
}

func (r *Router) Lookup(ctx context.Context, domain string, strategy dns.DomainStrategy) ([]netip.Addr, error) {
	r.dnsLogger.DebugContext(ctx, "lookup domain ", domain)
	ctx, metadata := adapter.AppendContext(ctx)
	metadata.Domain = domain
	ctx, transport, transportStrategy := r.matchDNS(ctx)
	if strategy == dns.DomainStrategyAsIS {
		strategy = transportStrategy
	}
	ctx, cancel := context.WithTimeout(ctx, C.DNSTimeout)
	defer cancel()
	addrs, err := r.dnsClient.Lookup(ctx, transport, domain, strategy)
	if len(addrs) > 0 {
		r.dnsLogger.InfoContext(ctx, "lookup succeed for ", domain, ": ", strings.Join(F.MapToString(addrs), " "))
	} else {
		r.dnsLogger.ErrorContext(ctx, E.Cause(err, "lookup failed for ", domain))
		if err == nil {
			err = dns.RCodeNameError
		}
	}
	return addrs, err
}

func (r *Router) LookupDefault(ctx context.Context, domain string) ([]netip.Addr, error) {
	return r.Lookup(ctx, domain, dns.DomainStrategyAsIS)
}

func LogDNSAnswers(logger log.ContextLogger, ctx context.Context, domain string, answers []mDNS.RR) {
	for _, answer := range answers {
		logger.InfoContext(ctx, "exchanged ", domain, " ", mDNS.Type(answer.Header().Rrtype).String(), " ", formatQuestion(answer.String()))
	}
}

func fqdnToDomain(fqdn string) string {
	if mDNS.IsFqdn(fqdn) {
		return fqdn[:len(fqdn)-1]
	}
	return fqdn
}

func formatQuestion(string string) string {
	if strings.HasPrefix(string, ";") {
		string = string[1:]
	}
	string = strings.ReplaceAll(string, "\t", " ")
	for strings.Contains(string, "  ") {
		string = strings.ReplaceAll(string, "  ", " ")
	}
	return string
}
