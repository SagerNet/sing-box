package route

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	R "github.com/sagernet/sing-box/route/rule"
	"github.com/sagernet/sing-dns"
	tun "github.com/sagernet/sing-tun"
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

func (r *Router) matchDNS(ctx context.Context, allowFakeIP bool, ruleIndex int, isAddressQuery bool) (dns.Transport, dns.QueryOptions, adapter.DNSRule, int) {
	metadata := adapter.ContextFrom(ctx)
	if metadata == nil {
		panic("no context")
	}
	var options dns.QueryOptions
	if ruleIndex < len(r.dnsRules) {
		dnsRules := r.dnsRules
		if ruleIndex != -1 {
			dnsRules = dnsRules[ruleIndex+1:]
		}
		for currentRuleIndex, currentRule := range dnsRules {
			if currentRule.WithAddressLimit() && !isAddressQuery {
				continue
			}
			metadata.ResetRuleCache()
			if currentRule.Match(metadata) {
				displayRuleIndex := currentRuleIndex
				if displayRuleIndex != -1 {
					displayRuleIndex += displayRuleIndex + 1
				}
				ruleDescription := currentRule.String()
				if ruleDescription != "" {
					r.logger.DebugContext(ctx, "match[", displayRuleIndex, "] ", currentRule, " => ", currentRule.Action())
				} else {
					r.logger.DebugContext(ctx, "match[", displayRuleIndex, "] => ", currentRule.Action())
				}
				switch action := currentRule.Action().(type) {
				case *R.RuleActionDNSRoute:
					transport, loaded := r.transportMap[action.Server]
					if !loaded {
						r.dnsLogger.ErrorContext(ctx, "transport not found: ", action.Server)
						continue
					}
					_, isFakeIP := transport.(adapter.FakeIPTransport)
					if isFakeIP && !allowFakeIP {
						continue
					}
					if isFakeIP || action.DisableCache {
						options.DisableCache = true
					}
					if action.RewriteTTL != nil {
						options.RewriteTTL = action.RewriteTTL
					}
					if action.ClientSubnet.IsValid() {
						options.ClientSubnet = action.ClientSubnet
					}
					if domainStrategy, dsLoaded := r.transportDomainStrategy[transport]; dsLoaded {
						options.Strategy = domainStrategy
					} else {
						options.Strategy = r.defaultDomainStrategy
					}
					r.logger.DebugContext(ctx, "match[", displayRuleIndex, "] => ", currentRule.Action())
					return transport, options, currentRule, currentRuleIndex
				case *R.RuleActionDNSRouteOptions:
					if action.DisableCache {
						options.DisableCache = true
					}
					if action.RewriteTTL != nil {
						options.RewriteTTL = action.RewriteTTL
					}
					if action.ClientSubnet.IsValid() {
						options.ClientSubnet = action.ClientSubnet
					}
					r.logger.DebugContext(ctx, "match[", displayRuleIndex, "] => ", currentRule.Action())
				case *R.RuleActionReject:
					r.logger.DebugContext(ctx, "match[", displayRuleIndex, "] => ", currentRule.Action())
					return nil, options, currentRule, currentRuleIndex
				}
			}
		}
	}
	if domainStrategy, dsLoaded := r.transportDomainStrategy[r.defaultTransport]; dsLoaded {
		options.Strategy = domainStrategy
	} else {
		options.Strategy = r.defaultDomainStrategy
	}
	return r.defaultTransport, options, nil, -1
}

func (r *Router) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	if len(message.Question) > 0 {
		r.dnsLogger.DebugContext(ctx, "exchange ", formatQuestion(message.Question[0].String()))
	}
	var (
		response  *mDNS.Msg
		cached    bool
		transport dns.Transport
		err       error
	)
	response, cached = r.dnsClient.ExchangeCache(ctx, message)
	if !cached {
		var metadata *adapter.InboundContext
		ctx, metadata = adapter.ExtendContext(ctx)
		metadata.Destination = M.Socksaddr{}
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
		var (
			options   dns.QueryOptions
			rule      adapter.DNSRule
			ruleIndex int
		)
		ruleIndex = -1
		for {
			dnsCtx := adapter.OverrideContext(ctx)
			var addressLimit bool
			transport, options, rule, ruleIndex = r.matchDNS(ctx, true, ruleIndex, isAddressQuery(message))
			if rule != nil {
				switch action := rule.Action().(type) {
				case *R.RuleActionReject:
					switch action.Method {
					case C.RuleActionRejectMethodDefault:
						return dns.FixedResponse(message.Id, message.Question[0], nil, 0), nil
					case C.RuleActionRejectMethodDrop:
						return nil, tun.ErrDrop
					}
				}
			}
			if rule != nil && rule.WithAddressLimit() {
				addressLimit = true
				response, err = r.dnsClient.ExchangeWithResponseCheck(dnsCtx, transport, message, options, func(response *mDNS.Msg) bool {
					addresses, addrErr := dns.MessageToAddresses(response)
					if addrErr != nil {
						return false
					}
					metadata.DestinationAddresses = addresses
					return rule.MatchAddressLimit(metadata)
				})
			} else {
				addressLimit = false
				response, err = r.dnsClient.Exchange(dnsCtx, transport, message, options)
			}
			var rejected bool
			if err != nil {
				if errors.Is(err, dns.ErrResponseRejectedCached) {
					rejected = true
					r.dnsLogger.DebugContext(ctx, E.Cause(err, "response rejected for ", formatQuestion(message.Question[0].String())), " (cached)")
				} else if errors.Is(err, dns.ErrResponseRejected) {
					rejected = true
					r.dnsLogger.DebugContext(ctx, E.Cause(err, "response rejected for ", formatQuestion(message.Question[0].String())))
				} else if len(message.Question) > 0 {
					r.dnsLogger.ErrorContext(ctx, E.Cause(err, "exchange failed for ", formatQuestion(message.Question[0].String())))
				} else {
					r.dnsLogger.ErrorContext(ctx, E.Cause(err, "exchange failed for <empty query>"))
				}
			}
			if addressLimit && rejected {
				continue
			}
			break
		}
	}
	if err != nil {
		return nil, err
	}
	if r.dnsReverseMapping != nil && len(message.Question) > 0 && response != nil && len(response.Answer) > 0 {
		if _, isFakeIP := transport.(adapter.FakeIPTransport); !isFakeIP {
			for _, answer := range response.Answer {
				switch record := answer.(type) {
				case *mDNS.A:
					r.dnsReverseMapping.Save(M.AddrFromIP(record.A), fqdnToDomain(record.Hdr.Name), int(record.Hdr.Ttl))
				case *mDNS.AAAA:
					r.dnsReverseMapping.Save(M.AddrFromIP(record.AAAA), fqdnToDomain(record.Hdr.Name), int(record.Hdr.Ttl))
				}
			}
		}
	}
	return response, nil
}

func (r *Router) Lookup(ctx context.Context, domain string, strategy dns.DomainStrategy) ([]netip.Addr, error) {
	var (
		responseAddrs []netip.Addr
		cached        bool
		err           error
	)
	printResult := func() {
		if err != nil {
			if errors.Is(err, dns.ErrResponseRejectedCached) {
				r.dnsLogger.DebugContext(ctx, "response rejected for ", domain, " (cached)")
			} else if errors.Is(err, dns.ErrResponseRejected) {
				r.dnsLogger.DebugContext(ctx, "response rejected for ", domain)
			} else {
				r.dnsLogger.ErrorContext(ctx, E.Cause(err, "lookup failed for ", domain))
			}
		} else if len(responseAddrs) == 0 {
			r.dnsLogger.ErrorContext(ctx, "lookup failed for ", domain, ": empty result")
			err = dns.RCodeNameError
		}
	}
	responseAddrs, cached = r.dnsClient.LookupCache(ctx, domain, strategy)
	if cached {
		if len(responseAddrs) == 0 {
			return nil, dns.RCodeNameError
		}
		return responseAddrs, nil
	}
	r.dnsLogger.DebugContext(ctx, "lookup domain ", domain)
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Destination = M.Socksaddr{}
	metadata.Domain = domain
	if metadata.DNSServer != "" {
		transport, loaded := r.transportMap[metadata.DNSServer]
		if !loaded {
			return nil, E.New("transport not found: ", metadata.DNSServer)
		}
		if strategy == dns.DomainStrategyAsIS {
			if transportDomainStrategy, loaded := r.transportDomainStrategy[transport]; loaded {
				strategy = transportDomainStrategy
			} else {
				strategy = r.defaultDomainStrategy
			}
		}
		responseAddrs, err = r.dnsClient.Lookup(ctx, transport, domain, dns.QueryOptions{Strategy: strategy})
	} else {
		var (
			transport dns.Transport
			options   dns.QueryOptions
			rule      adapter.DNSRule
			ruleIndex int
		)
		ruleIndex = -1
		for {
			dnsCtx := adapter.OverrideContext(ctx)
			var addressLimit bool
			transport, options, rule, ruleIndex = r.matchDNS(ctx, false, ruleIndex, true)
			if strategy != dns.DomainStrategyAsIS {
				options.Strategy = strategy
			}
			if rule != nil {
				switch action := rule.Action().(type) {
				case *R.RuleActionReject:
					switch action.Method {
					case C.RuleActionRejectMethodDefault:
						return nil, nil
					case C.RuleActionRejectMethodDrop:
						return nil, tun.ErrDrop
					}
				}
			}
			if rule != nil && rule.WithAddressLimit() {
				addressLimit = true
				responseAddrs, err = r.dnsClient.LookupWithResponseCheck(dnsCtx, transport, domain, options, func(responseAddrs []netip.Addr) bool {
					metadata.DestinationAddresses = responseAddrs
					return rule.MatchAddressLimit(metadata)
				})
			} else {
				addressLimit = false
				responseAddrs, err = r.dnsClient.Lookup(dnsCtx, transport, domain, options)
			}
			if !addressLimit || err == nil {
				break
			}
			printResult()
		}
	}
	printResult()
	if len(responseAddrs) > 0 {
		r.dnsLogger.InfoContext(ctx, "lookup succeed for ", domain, ": ", strings.Join(F.MapToString(responseAddrs), " "))
	}
	return responseAddrs, err
}

func (r *Router) LookupDefault(ctx context.Context, domain string) ([]netip.Addr, error) {
	return r.Lookup(ctx, domain, dns.DomainStrategyAsIS)
}

func (r *Router) ClearDNSCache() {
	r.dnsClient.ClearCache()
	if r.platformInterface != nil {
		r.platformInterface.ClearDNSCache()
	}
}

func isAddressQuery(message *mDNS.Msg) bool {
	for _, question := range message.Question {
		if question.Qtype == mDNS.TypeA || question.Qtype == mDNS.TypeAAAA || question.Qtype == mDNS.TypeHTTPS {
			return true
		}
	}
	return false
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
