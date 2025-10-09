package dns

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/compatible"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/task"
	"github.com/sagernet/sing/contrab/freelru"
	"github.com/sagernet/sing/contrab/maphash"

	"github.com/miekg/dns"
)

var (
	ErrNoRawSupport           = E.New("no raw query support by current transport")
	ErrNotCached              = E.New("not cached")
	ErrResponseRejected       = E.New("response rejected")
	ErrResponseRejectedCached = E.Extend(ErrResponseRejected, "cached")
)

var _ adapter.DNSClient = (*Client)(nil)

type Client struct {
	timeout            time.Duration
	disableCache       bool
	disableExpire      bool
	independentCache   bool
	clientSubnet       netip.Prefix
	rdrc               adapter.RDRCStore
	initRDRCFunc       func() adapter.RDRCStore
	logger             logger.ContextLogger
	cache              freelru.Cache[dns.Question, *dns.Msg]
	cacheLock          compatible.Map[dns.Question, chan struct{}]
	transportCache     freelru.Cache[transportCacheKey, *dns.Msg]
	transportCacheLock compatible.Map[dns.Question, chan struct{}]
}

type ClientOptions struct {
	Timeout          time.Duration
	DisableCache     bool
	DisableExpire    bool
	IndependentCache bool
	CacheCapacity    uint32
	ClientSubnet     netip.Prefix
	RDRC             func() adapter.RDRCStore
	Logger           logger.ContextLogger
}

func NewClient(options ClientOptions) *Client {
	client := &Client{
		timeout:          options.Timeout,
		disableCache:     options.DisableCache,
		disableExpire:    options.DisableExpire,
		independentCache: options.IndependentCache,
		clientSubnet:     options.ClientSubnet,
		initRDRCFunc:     options.RDRC,
		logger:           options.Logger,
	}
	if client.timeout == 0 {
		client.timeout = C.DNSTimeout
	}
	cacheCapacity := options.CacheCapacity
	if cacheCapacity < 1024 {
		cacheCapacity = 1024
	}
	if !client.disableCache {
		if !client.independentCache {
			client.cache = common.Must1(freelru.NewSharded[dns.Question, *dns.Msg](cacheCapacity, maphash.NewHasher[dns.Question]().Hash32))
		} else {
			client.transportCache = common.Must1(freelru.NewSharded[transportCacheKey, *dns.Msg](cacheCapacity, maphash.NewHasher[transportCacheKey]().Hash32))
		}
	}
	return client
}

type transportCacheKey struct {
	dns.Question
	transportTag string
}

func (c *Client) Start() {
	if c.initRDRCFunc != nil {
		c.rdrc = c.initRDRCFunc()
	}
}

func extractNegativeTTL(response *dns.Msg) (uint32, bool) {
	for _, record := range response.Ns {
		if soa, isSOA := record.(*dns.SOA); isSOA {
			soaTTL := soa.Header().Ttl
			soaMinimum := soa.Minttl
			if soaTTL < soaMinimum {
				return soaTTL, true
			}
			return soaMinimum, true
		}
	}
	return 0, false
}

func (c *Client) Exchange(ctx context.Context, transport adapter.DNSTransport, message *dns.Msg, options adapter.DNSQueryOptions, responseChecker func(responseAddrs []netip.Addr) bool) (*dns.Msg, error) {
	if len(message.Question) == 0 {
		if c.logger != nil {
			c.logger.WarnContext(ctx, "bad question size: ", len(message.Question))
		}
		return FixedResponseStatus(message, dns.RcodeFormatError), nil
	}
	question := message.Question[0]
	if question.Qtype == dns.TypeA && options.Strategy == C.DomainStrategyIPv6Only || question.Qtype == dns.TypeAAAA && options.Strategy == C.DomainStrategyIPv4Only {
		if c.logger != nil {
			c.logger.DebugContext(ctx, "strategy rejected")
		}
		return FixedResponseStatus(message, dns.RcodeSuccess), nil
	}
	clientSubnet := options.ClientSubnet
	if !clientSubnet.IsValid() {
		clientSubnet = c.clientSubnet
	}
	if clientSubnet.IsValid() {
		message = SetClientSubnet(message, clientSubnet)
	}

	isSimpleRequest := len(message.Question) == 1 &&
		len(message.Ns) == 0 &&
		(len(message.Extra) == 0 || len(message.Extra) == 1 &&
			message.Extra[0].Header().Rrtype == dns.TypeOPT &&
			message.Extra[0].Header().Class > 0 &&
			message.Extra[0].Header().Ttl == 0 &&
			len(message.Extra[0].(*dns.OPT).Option) == 0) &&
		!options.ClientSubnet.IsValid()
	disableCache := !isSimpleRequest || c.disableCache || options.DisableCache
	if !disableCache {
		if c.cache != nil {
			cond, loaded := c.cacheLock.LoadOrStore(question, make(chan struct{}))
			if loaded {
				<-cond
			} else {
				defer func() {
					c.cacheLock.Delete(question)
					close(cond)
				}()
			}
		} else if c.transportCache != nil {
			cond, loaded := c.transportCacheLock.LoadOrStore(question, make(chan struct{}))
			if loaded {
				<-cond
			} else {
				defer func() {
					c.transportCacheLock.Delete(question)
					close(cond)
				}()
			}
		}
		response, ttl := c.loadResponse(question, transport)
		if response != nil {
			logCachedResponse(c.logger, ctx, response, ttl)
			response.Id = message.Id
			return response, nil
		}
	}

	messageId := message.Id
	contextTransport, clientSubnetLoaded := transportTagFromContext(ctx)
	if clientSubnetLoaded && transport.Tag() == contextTransport {
		return nil, E.New("DNS query loopback in transport[", contextTransport, "]")
	}
	ctx = contextWithTransportTag(ctx, transport.Tag())
	if !disableCache && responseChecker != nil && c.rdrc != nil {
		rejected := c.rdrc.LoadRDRC(transport.Tag(), question.Name, question.Qtype)
		if rejected {
			return nil, ErrResponseRejectedCached
		}
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	response, err := transport.Exchange(ctx, message)
	cancel()
	if err != nil {
		var rcodeError RcodeError
		if errors.As(err, &rcodeError) {
			response = FixedResponseStatus(message, int(rcodeError))
		} else {
			return nil, err
		}
	}
	/*if question.Qtype == dns.TypeA || question.Qtype == dns.TypeAAAA {
		validResponse := response
	loop:
		for {
			var (
				addresses  int
				queryCNAME string
			)
			for _, rawRR := range validResponse.Answer {
				switch rr := rawRR.(type) {
				case *dns.A:
					break loop
				case *dns.AAAA:
					break loop
				case *dns.CNAME:
					queryCNAME = rr.Target
				}
			}
			if queryCNAME == "" {
				break
			}
			exMessage := *message
			exMessage.Question = []dns.Question{{
				Name:  queryCNAME,
				Qtype: question.Qtype,
			}}
			validResponse, err = c.Exchange(ctx, transport, &exMessage, options, responseChecker)
			if err != nil {
				return nil, err
			}
		}
		if validResponse != response {
			response.Answer = append(response.Answer, validResponse.Answer...)
		}
	}*/
	disableCache = disableCache || (response.Rcode != dns.RcodeSuccess && response.Rcode != dns.RcodeNameError)
	if responseChecker != nil {
		var rejected bool
		// TODO: add accept_any rule and support to check response instead of addresses
		if response.Rcode != dns.RcodeSuccess || len(response.Answer) == 0 {
			rejected = true
		} else {
			rejected = !responseChecker(MessageToAddresses(response))
		}
		if rejected {
			if !disableCache && c.rdrc != nil {
				c.rdrc.SaveRDRCAsync(transport.Tag(), question.Name, question.Qtype, c.logger)
			}
			logRejectedResponse(c.logger, ctx, response)
			return response, ErrResponseRejected
		}
	}
	if question.Qtype == dns.TypeHTTPS {
		if options.Strategy == C.DomainStrategyIPv4Only || options.Strategy == C.DomainStrategyIPv6Only {
			for _, rr := range response.Answer {
				https, isHTTPS := rr.(*dns.HTTPS)
				if !isHTTPS {
					continue
				}
				content := https.SVCB
				content.Value = common.Filter(content.Value, func(it dns.SVCBKeyValue) bool {
					if options.Strategy == C.DomainStrategyIPv4Only {
						return it.Key() != dns.SVCB_IPV6HINT
					} else {
						return it.Key() != dns.SVCB_IPV4HINT
					}
				})
				https.SVCB = content
			}
		}
	}
	var timeToLive uint32
	if len(response.Answer) == 0 {
		if soaTTL, hasSOA := extractNegativeTTL(response); hasSOA {
			timeToLive = soaTTL
		}
	}
	if timeToLive == 0 {
		for _, recordList := range [][]dns.RR{response.Answer, response.Ns, response.Extra} {
			for _, record := range recordList {
				if timeToLive == 0 || record.Header().Ttl > 0 && record.Header().Ttl < timeToLive {
					timeToLive = record.Header().Ttl
				}
			}
		}
	}
	if options.RewriteTTL != nil {
		timeToLive = *options.RewriteTTL
	}
	for _, recordList := range [][]dns.RR{response.Answer, response.Ns, response.Extra} {
		for _, record := range recordList {
			record.Header().Ttl = timeToLive
		}
	}
	if !disableCache {
		c.storeCache(transport, question, response, timeToLive)
	}
	response.Id = messageId
	requestEDNSOpt := message.IsEdns0()
	responseEDNSOpt := response.IsEdns0()
	if responseEDNSOpt != nil && (requestEDNSOpt == nil || requestEDNSOpt.Version() < responseEDNSOpt.Version()) {
		response.Extra = common.Filter(response.Extra, func(it dns.RR) bool {
			return it.Header().Rrtype != dns.TypeOPT
		})
		if requestEDNSOpt != nil {
			response.SetEdns0(responseEDNSOpt.UDPSize(), responseEDNSOpt.Do())
		}
	}
	logExchangedResponse(c.logger, ctx, response, timeToLive)
	return response, nil
}

func (c *Client) Lookup(ctx context.Context, transport adapter.DNSTransport, domain string, options adapter.DNSQueryOptions, responseChecker func(responseAddrs []netip.Addr) bool) ([]netip.Addr, error) {
	domain = FqdnToDomain(domain)
	dnsName := dns.Fqdn(domain)
	var strategy C.DomainStrategy
	if options.LookupStrategy != C.DomainStrategyAsIS {
		strategy = options.LookupStrategy
	} else {
		strategy = options.Strategy
	}
	if strategy == C.DomainStrategyIPv4Only {
		return c.lookupToExchange(ctx, transport, dnsName, dns.TypeA, options, responseChecker)
	} else if strategy == C.DomainStrategyIPv6Only {
		return c.lookupToExchange(ctx, transport, dnsName, dns.TypeAAAA, options, responseChecker)
	}
	var response4 []netip.Addr
	var response6 []netip.Addr
	var group task.Group
	group.Append("exchange4", func(ctx context.Context) error {
		response, err := c.lookupToExchange(ctx, transport, dnsName, dns.TypeA, options, responseChecker)
		if err != nil {
			return err
		}
		response4 = response
		return nil
	})
	group.Append("exchange6", func(ctx context.Context) error {
		response, err := c.lookupToExchange(ctx, transport, dnsName, dns.TypeAAAA, options, responseChecker)
		if err != nil {
			return err
		}
		response6 = response
		return nil
	})
	err := group.Run(ctx)
	if len(response4) == 0 && len(response6) == 0 {
		return nil, err
	}
	return sortAddresses(response4, response6, strategy), nil
}

func (c *Client) ClearCache() {
	if c.cache != nil {
		c.cache.Purge()
	} else if c.transportCache != nil {
		c.transportCache.Purge()
	}
}

func (c *Client) LookupCache(domain string, strategy C.DomainStrategy) ([]netip.Addr, bool) {
	if c.disableCache || c.independentCache {
		return nil, false
	}
	if dns.IsFqdn(domain) {
		domain = domain[:len(domain)-1]
	}
	dnsName := dns.Fqdn(domain)
	if strategy == C.DomainStrategyIPv4Only {
		addresses, err := c.questionCache(dns.Question{
			Name:   dnsName,
			Qtype:  dns.TypeA,
			Qclass: dns.ClassINET,
		}, nil)
		if err != ErrNotCached {
			return addresses, true
		}
	} else if strategy == C.DomainStrategyIPv6Only {
		addresses, err := c.questionCache(dns.Question{
			Name:   dnsName,
			Qtype:  dns.TypeAAAA,
			Qclass: dns.ClassINET,
		}, nil)
		if err != ErrNotCached {
			return addresses, true
		}
	} else {
		response4, _ := c.loadResponse(dns.Question{
			Name:   dnsName,
			Qtype:  dns.TypeA,
			Qclass: dns.ClassINET,
		}, nil)
		if response4 == nil {
			return nil, false
		}
		response6, _ := c.loadResponse(dns.Question{
			Name:   dnsName,
			Qtype:  dns.TypeAAAA,
			Qclass: dns.ClassINET,
		}, nil)
		if response6 == nil {
			return nil, false
		}
		return sortAddresses(MessageToAddresses(response4), MessageToAddresses(response6), strategy), true
	}
	return nil, false
}

func (c *Client) ExchangeCache(ctx context.Context, message *dns.Msg) (*dns.Msg, bool) {
	if c.disableCache || c.independentCache || len(message.Question) != 1 {
		return nil, false
	}
	question := message.Question[0]
	response, ttl := c.loadResponse(question, nil)
	if response == nil {
		return nil, false
	}
	logCachedResponse(c.logger, ctx, response, ttl)
	response.Id = message.Id
	return response, true
}

func sortAddresses(response4 []netip.Addr, response6 []netip.Addr, strategy C.DomainStrategy) []netip.Addr {
	if strategy == C.DomainStrategyPreferIPv6 {
		return append(response6, response4...)
	} else {
		return append(response4, response6...)
	}
}

func (c *Client) storeCache(transport adapter.DNSTransport, question dns.Question, message *dns.Msg, timeToLive uint32) {
	if timeToLive == 0 {
		return
	}
	if c.disableExpire {
		if !c.independentCache {
			c.cache.Add(question, message)
		} else {
			c.transportCache.Add(transportCacheKey{
				Question:     question,
				transportTag: transport.Tag(),
			}, message)
		}
	} else {
		if !c.independentCache {
			c.cache.AddWithLifetime(question, message, time.Second*time.Duration(timeToLive))
		} else {
			c.transportCache.AddWithLifetime(transportCacheKey{
				Question:     question,
				transportTag: transport.Tag(),
			}, message, time.Second*time.Duration(timeToLive))
		}
	}
}

func (c *Client) lookupToExchange(ctx context.Context, transport adapter.DNSTransport, name string, qType uint16, options adapter.DNSQueryOptions, responseChecker func(responseAddrs []netip.Addr) bool) ([]netip.Addr, error) {
	question := dns.Question{
		Name:   name,
		Qtype:  qType,
		Qclass: dns.ClassINET,
	}
	disableCache := c.disableCache || options.DisableCache
	if !disableCache {
		cachedAddresses, err := c.questionCache(question, transport)
		if err != ErrNotCached {
			return cachedAddresses, err
		}
	}
	message := dns.Msg{
		MsgHdr: dns.MsgHdr{
			RecursionDesired: true,
		},
		Question: []dns.Question{question},
	}
	response, err := c.Exchange(ctx, transport, &message, options, responseChecker)
	if err != nil {
		return nil, err
	}
	if response.Rcode != dns.RcodeSuccess {
		return nil, RcodeError(response.Rcode)
	}
	return MessageToAddresses(response), nil
}

func (c *Client) questionCache(question dns.Question, transport adapter.DNSTransport) ([]netip.Addr, error) {
	response, _ := c.loadResponse(question, transport)
	if response == nil {
		return nil, ErrNotCached
	}
	if response.Rcode != dns.RcodeSuccess {
		return nil, RcodeError(response.Rcode)
	}
	return MessageToAddresses(response), nil
}

func (c *Client) loadResponse(question dns.Question, transport adapter.DNSTransport) (*dns.Msg, int) {
	var (
		response *dns.Msg
		loaded   bool
	)
	if c.disableExpire {
		if !c.independentCache {
			response, loaded = c.cache.Get(question)
		} else {
			response, loaded = c.transportCache.Get(transportCacheKey{
				Question:     question,
				transportTag: transport.Tag(),
			})
		}
		if !loaded {
			return nil, 0
		}
		return response.Copy(), 0
	} else {
		var expireAt time.Time
		if !c.independentCache {
			response, expireAt, loaded = c.cache.GetWithLifetime(question)
		} else {
			response, expireAt, loaded = c.transportCache.GetWithLifetime(transportCacheKey{
				Question:     question,
				transportTag: transport.Tag(),
			})
		}
		if !loaded {
			return nil, 0
		}
		timeNow := time.Now()
		if timeNow.After(expireAt) {
			if !c.independentCache {
				c.cache.Remove(question)
			} else {
				c.transportCache.Remove(transportCacheKey{
					Question:     question,
					transportTag: transport.Tag(),
				})
			}
			return nil, 0
		}
		var originTTL int
		for _, recordList := range [][]dns.RR{response.Answer, response.Ns, response.Extra} {
			for _, record := range recordList {
				if originTTL == 0 || record.Header().Ttl > 0 && int(record.Header().Ttl) < originTTL {
					originTTL = int(record.Header().Ttl)
				}
			}
		}
		nowTTL := int(expireAt.Sub(timeNow).Seconds())
		if nowTTL < 0 {
			nowTTL = 0
		}
		response = response.Copy()
		if originTTL > 0 {
			duration := uint32(originTTL - nowTTL)
			for _, recordList := range [][]dns.RR{response.Answer, response.Ns, response.Extra} {
				for _, record := range recordList {
					record.Header().Ttl = record.Header().Ttl - duration
				}
			}
		} else {
			for _, recordList := range [][]dns.RR{response.Answer, response.Ns, response.Extra} {
				for _, record := range recordList {
					record.Header().Ttl = uint32(nowTTL)
				}
			}
		}
		return response, nowTTL
	}
}

func MessageToAddresses(response *dns.Msg) []netip.Addr {
	if response == nil || response.Rcode != dns.RcodeSuccess {
		return nil
	}
	addresses := make([]netip.Addr, 0, len(response.Answer))
	for _, rawAnswer := range response.Answer {
		switch answer := rawAnswer.(type) {
		case *dns.A:
			addresses = append(addresses, M.AddrFromIP(answer.A))
		case *dns.AAAA:
			addresses = append(addresses, M.AddrFromIP(answer.AAAA))
		case *dns.HTTPS:
			for _, value := range answer.SVCB.Value {
				if value.Key() == dns.SVCB_IPV4HINT || value.Key() == dns.SVCB_IPV6HINT {
					addresses = append(addresses, common.Map(strings.Split(value.String(), ","), M.ParseAddr)...)
				}
			}
		}
	}
	return addresses
}

func wrapError(err error) error {
	switch dnsErr := err.(type) {
	case *net.DNSError:
		if dnsErr.IsNotFound {
			return RcodeNameError
		}
	case *net.AddrError:
		return RcodeNameError
	}
	return err
}

type transportKey struct{}

func contextWithTransportTag(ctx context.Context, transportTag string) context.Context {
	return context.WithValue(ctx, transportKey{}, transportTag)
}

func transportTagFromContext(ctx context.Context) (string, bool) {
	value, loaded := ctx.Value(transportKey{}).(string)
	return value, loaded
}

func FixedResponseStatus(message *dns.Msg, rcode int) *dns.Msg {
	return &dns.Msg{
		MsgHdr: dns.MsgHdr{
			Id:                 message.Id,
			Response:           true,
			Authoritative:      true,
			RecursionDesired:   true,
			RecursionAvailable: true,
			Rcode:              rcode,
		},
		Question: message.Question,
	}
}

func FixedResponse(id uint16, question dns.Question, addresses []netip.Addr, timeToLive uint32) *dns.Msg {
	response := dns.Msg{
		MsgHdr: dns.MsgHdr{
			Id:                 id,
			Response:           true,
			Authoritative:      true,
			RecursionDesired:   true,
			RecursionAvailable: true,
			Rcode:              dns.RcodeSuccess,
		},
		Question: []dns.Question{question},
	}
	for _, address := range addresses {
		if address.Is4() && question.Qtype == dns.TypeA {
			response.Answer = append(response.Answer, &dns.A{
				Hdr: dns.RR_Header{
					Name:   question.Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    timeToLive,
				},
				A: address.AsSlice(),
			})
		} else if address.Is6() && question.Qtype == dns.TypeAAAA {
			response.Answer = append(response.Answer, &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   question.Name,
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    timeToLive,
				},
				AAAA: address.AsSlice(),
			})
		}
	}
	return &response
}

func FixedResponseCNAME(id uint16, question dns.Question, record string, timeToLive uint32) *dns.Msg {
	response := dns.Msg{
		MsgHdr: dns.MsgHdr{
			Id:                 id,
			Response:           true,
			Authoritative:      true,
			RecursionDesired:   true,
			RecursionAvailable: true,
			Rcode:              dns.RcodeSuccess,
		},
		Question: []dns.Question{question},
		Answer: []dns.RR{
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   question.Name,
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
					Ttl:    timeToLive,
				},
				Target: record,
			},
		},
	}
	return &response
}

func FixedResponseTXT(id uint16, question dns.Question, records []string, timeToLive uint32) *dns.Msg {
	response := dns.Msg{
		MsgHdr: dns.MsgHdr{
			Id:                 id,
			Response:           true,
			Authoritative:      true,
			RecursionDesired:   true,
			RecursionAvailable: true,
			Rcode:              dns.RcodeSuccess,
		},
		Question: []dns.Question{question},
		Answer: []dns.RR{
			&dns.TXT{
				Hdr: dns.RR_Header{
					Name:   question.Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    timeToLive,
				},
				Txt: records,
			},
		},
	}
	return &response
}

func FixedResponseMX(id uint16, question dns.Question, records []*net.MX, timeToLive uint32) *dns.Msg {
	response := dns.Msg{
		MsgHdr: dns.MsgHdr{
			Id:                 id,
			Response:           true,
			Authoritative:      true,
			RecursionDesired:   true,
			RecursionAvailable: true,
			Rcode:              dns.RcodeSuccess,
		},
		Question: []dns.Question{question},
	}
	for _, record := range records {
		response.Answer = append(response.Answer, &dns.MX{
			Hdr: dns.RR_Header{
				Name:   question.Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    timeToLive,
			},
			Preference: record.Pref,
			Mx:         record.Host,
		})
	}
	return &response
}
