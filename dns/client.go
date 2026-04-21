package dns

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/compatible"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
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
	ctx               context.Context
	timeout           time.Duration
	disableCache      bool
	disableExpire     bool
	optimisticTimeout time.Duration
	cacheCapacity     uint32
	clientSubnet      netip.Prefix
	rdrc              adapter.RDRCStore
	initRDRCFunc      func() adapter.RDRCStore
	dnsCache          adapter.DNSCacheStore
	initDNSCacheFunc  func() adapter.DNSCacheStore
	logger            logger.ContextLogger
	cache             freelru.Cache[dnsCacheKey, *dns.Msg]
	cacheLock         compatible.Map[dnsCacheKey, chan struct{}]
	backgroundRefresh compatible.Map[dnsCacheKey, struct{}]
}

type ClientOptions struct {
	Context           context.Context
	Timeout           time.Duration
	DisableCache      bool
	DisableExpire     bool
	OptimisticTimeout time.Duration
	CacheCapacity     uint32
	ClientSubnet      netip.Prefix
	RDRC              func() adapter.RDRCStore
	DNSCache          func() adapter.DNSCacheStore
	Logger            logger.ContextLogger
}

func NewClient(options ClientOptions) *Client {
	cacheCapacity := options.CacheCapacity
	if cacheCapacity < 1024 {
		cacheCapacity = 1024
	}
	client := &Client{
		ctx:               options.Context,
		timeout:           options.Timeout,
		disableCache:      options.DisableCache,
		disableExpire:     options.DisableExpire,
		optimisticTimeout: options.OptimisticTimeout,
		cacheCapacity:     cacheCapacity,
		clientSubnet:      options.ClientSubnet,
		initRDRCFunc:      options.RDRC,
		initDNSCacheFunc:  options.DNSCache,
		logger:            options.Logger,
	}
	if client.timeout == 0 {
		client.timeout = C.DNSTimeout
	}
	if !client.disableCache && client.initDNSCacheFunc == nil {
		client.initializeMemoryCache()
	}
	return client
}

type dnsCacheKey struct {
	dns.Question
	transportTag string
}

func (c *Client) Start() {
	if c.initRDRCFunc != nil {
		c.rdrc = c.initRDRCFunc()
	}
	if c.initDNSCacheFunc != nil {
		c.dnsCache = c.initDNSCacheFunc()
	}
	if c.dnsCache == nil {
		c.initializeMemoryCache()
	}
}

func (c *Client) initializeMemoryCache() {
	if c.disableCache || c.cache != nil {
		return
	}
	c.cache = common.Must1(freelru.NewSharded[dnsCacheKey, *dns.Msg](c.cacheCapacity, maphash.NewHasher[dnsCacheKey]().Hash32))
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

func computeTimeToLive(response *dns.Msg) uint32 {
	var timeToLive uint32
	if len(response.Answer) == 0 {
		if soaTTL, hasSOA := extractNegativeTTL(response); hasSOA {
			return soaTTL
		}
	}
	for _, recordList := range [][]dns.RR{response.Answer, response.Ns, response.Extra} {
		for _, record := range recordList {
			if record.Header().Rrtype == dns.TypeOPT {
				continue
			}
			if timeToLive == 0 || record.Header().Ttl > 0 && record.Header().Ttl < timeToLive {
				timeToLive = record.Header().Ttl
			}
		}
	}
	return timeToLive
}

func normalizeTTL(response *dns.Msg, timeToLive uint32) {
	for _, recordList := range [][]dns.RR{response.Answer, response.Ns, response.Extra} {
		for _, record := range recordList {
			if record.Header().Rrtype == dns.TypeOPT {
				continue
			}
			record.Header().Ttl = timeToLive
		}
	}
}

func (c *Client) Exchange(ctx context.Context, transport adapter.DNSTransport, message *dns.Msg, options adapter.DNSQueryOptions, responseChecker func(response *dns.Msg) bool) (*dns.Msg, error) {
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
	message = c.prepareExchangeMessage(message, options)

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
		cacheKey := dnsCacheKey{Question: question, transportTag: transport.Tag()}
		cond, loaded := c.cacheLock.LoadOrStore(cacheKey, make(chan struct{}))
		if loaded {
			select {
			case <-cond:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		} else {
			defer func() {
				c.cacheLock.Delete(cacheKey)
				close(cond)
			}()
		}
		response, ttl, isStale := c.loadResponse(question, transport)
		if response != nil {
			if isStale && !options.DisableOptimisticCache {
				c.backgroundRefreshDNS(transport, question, message.Copy(), options, responseChecker)
				logOptimisticResponse(c.logger, ctx, response)
				response.Id = message.Id
				return response, nil
			} else if !isStale {
				logCachedResponse(c.logger, ctx, response, ttl)
				response.Id = message.Id
				return response, nil
			}
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
	response, err := c.exchangeToTransport(ctx, transport, message)
	if err != nil {
		return nil, err
	}
	disableCache = disableCache || (response.Rcode != dns.RcodeSuccess && response.Rcode != dns.RcodeNameError)
	if responseChecker != nil {
		var rejected bool
		if response.Rcode != dns.RcodeSuccess && response.Rcode != dns.RcodeNameError {
			rejected = true
		} else {
			rejected = !responseChecker(response)
		}
		if rejected {
			if !disableCache && c.rdrc != nil {
				c.rdrc.SaveRDRCAsync(transport.Tag(), question.Name, question.Qtype, c.logger)
			}
			logRejectedResponse(c.logger, ctx, response)
			return response, ErrResponseRejected
		}
	}
	timeToLive := applyResponseOptions(question, response, options)
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

func (c *Client) Lookup(ctx context.Context, transport adapter.DNSTransport, domain string, options adapter.DNSQueryOptions, responseChecker func(response *dns.Msg) bool) ([]netip.Addr, error) {
	domain = FqdnToDomain(domain)
	dnsName := dns.Fqdn(domain)
	var strategy C.DomainStrategy
	if options.LookupStrategy != C.DomainStrategyAsIS {
		strategy = options.LookupStrategy
	} else {
		strategy = options.Strategy
	}
	lookupOptions := options
	if options.LookupStrategy != C.DomainStrategyAsIS {
		lookupOptions.Strategy = strategy
	}
	if strategy == C.DomainStrategyIPv4Only {
		return c.lookupToExchange(ctx, transport, dnsName, dns.TypeA, lookupOptions, responseChecker)
	} else if strategy == C.DomainStrategyIPv6Only {
		return c.lookupToExchange(ctx, transport, dnsName, dns.TypeAAAA, lookupOptions, responseChecker)
	}
	var response4 []netip.Addr
	var response6 []netip.Addr
	var group task.Group
	group.Append("exchange4", func(ctx context.Context) error {
		response, err := c.lookupToExchange(ctx, transport, dnsName, dns.TypeA, lookupOptions, responseChecker)
		if err != nil {
			return err
		}
		response4 = response
		return nil
	})
	group.Append("exchange6", func(ctx context.Context) error {
		response, err := c.lookupToExchange(ctx, transport, dnsName, dns.TypeAAAA, lookupOptions, responseChecker)
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
	}
	if c.dnsCache != nil {
		err := c.dnsCache.ClearDNSCache()
		if err != nil && c.logger != nil {
			c.logger.Warn("clear DNS cache: ", err)
		}
	}
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
	if c.dnsCache != nil {
		packed, err := message.Pack()
		if err == nil {
			expireAt := time.Now().Add(time.Second * time.Duration(timeToLive))
			c.dnsCache.SaveDNSCacheAsync(transport.Tag(), question.Name, question.Qtype, packed, expireAt, c.logger)
		}
		return
	}
	if c.cache == nil {
		return
	}
	key := dnsCacheKey{Question: question, transportTag: transport.Tag()}
	if c.disableExpire {
		c.cache.Add(key, message.Copy())
	} else {
		c.cache.AddWithLifetime(key, message.Copy(), time.Second*time.Duration(timeToLive))
	}
}

func (c *Client) lookupToExchange(ctx context.Context, transport adapter.DNSTransport, name string, qType uint16, options adapter.DNSQueryOptions, responseChecker func(response *dns.Msg) bool) ([]netip.Addr, error) {
	question := dns.Question{
		Name:   name,
		Qtype:  qType,
		Qclass: dns.ClassINET,
	}
	message := dns.Msg{
		MsgHdr: dns.MsgHdr{
			RecursionDesired: true,
		},
		Question: []dns.Question{question},
	}
	disableCache := c.disableCache || options.DisableCache
	if !disableCache {
		cachedAddresses, err := c.questionCache(ctx, transport, &message, options, responseChecker)
		if err != ErrNotCached {
			return cachedAddresses, err
		}
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

func (c *Client) questionCache(ctx context.Context, transport adapter.DNSTransport, message *dns.Msg, options adapter.DNSQueryOptions, responseChecker func(response *dns.Msg) bool) ([]netip.Addr, error) {
	question := message.Question[0]
	response, _, isStale := c.loadResponse(question, transport)
	if response == nil {
		return nil, ErrNotCached
	}
	if isStale {
		if options.DisableOptimisticCache {
			return nil, ErrNotCached
		}
		c.backgroundRefreshDNS(transport, question, c.prepareExchangeMessage(message.Copy(), options), options, responseChecker)
		logOptimisticResponse(c.logger, ctx, response)
	}
	if response.Rcode != dns.RcodeSuccess {
		return nil, RcodeError(response.Rcode)
	}
	return MessageToAddresses(response), nil
}

func (c *Client) loadResponse(question dns.Question, transport adapter.DNSTransport) (*dns.Msg, int, bool) {
	if c.dnsCache != nil {
		return c.loadPersistentResponse(question, transport)
	}
	if c.cache == nil {
		return nil, 0, false
	}
	key := dnsCacheKey{Question: question, transportTag: transport.Tag()}
	if c.disableExpire {
		response, loaded := c.cache.Get(key)
		if !loaded {
			return nil, 0, false
		}
		return response.Copy(), 0, false
	}
	response, expireAt, loaded := c.cache.GetWithLifetimeNoExpire(key)
	if !loaded {
		return nil, 0, false
	}
	timeNow := time.Now()
	if timeNow.After(expireAt) {
		if c.optimisticTimeout > 0 && timeNow.Before(expireAt.Add(c.optimisticTimeout)) {
			response = response.Copy()
			normalizeTTL(response, 1)
			return response, 0, true
		}
		c.cache.Remove(key)
		return nil, 0, false
	}
	nowTTL := int(expireAt.Sub(timeNow).Seconds())
	if nowTTL < 0 {
		nowTTL = 0
	}
	response = response.Copy()
	normalizeTTL(response, uint32(nowTTL))
	return response, nowTTL, false
}

func (c *Client) loadPersistentResponse(question dns.Question, transport adapter.DNSTransport) (*dns.Msg, int, bool) {
	rawMessage, expireAt, loaded := c.dnsCache.LoadDNSCache(transport.Tag(), question.Name, question.Qtype)
	if !loaded {
		return nil, 0, false
	}
	response := new(dns.Msg)
	err := response.Unpack(rawMessage)
	if err != nil {
		return nil, 0, false
	}
	if c.disableExpire {
		return response, 0, false
	}
	timeNow := time.Now()
	if timeNow.After(expireAt) {
		if c.optimisticTimeout > 0 && timeNow.Before(expireAt.Add(c.optimisticTimeout)) {
			normalizeTTL(response, 1)
			return response, 0, true
		}
		return nil, 0, false
	}
	nowTTL := int(expireAt.Sub(timeNow).Seconds())
	if nowTTL < 0 {
		nowTTL = 0
	}
	normalizeTTL(response, uint32(nowTTL))
	return response, nowTTL, false
}

func applyResponseOptions(question dns.Question, response *dns.Msg, options adapter.DNSQueryOptions) uint32 {
	if question.Qtype == dns.TypeHTTPS && (options.Strategy == C.DomainStrategyIPv4Only || options.Strategy == C.DomainStrategyIPv6Only) {
		for _, rr := range response.Answer {
			https, isHTTPS := rr.(*dns.HTTPS)
			if !isHTTPS {
				continue
			}
			content := https.SVCB
			content.Value = common.Filter(content.Value, func(it dns.SVCBKeyValue) bool {
				if options.Strategy == C.DomainStrategyIPv4Only {
					return it.Key() != dns.SVCB_IPV6HINT
				}
				return it.Key() != dns.SVCB_IPV4HINT
			})
			https.SVCB = content
		}
	}
	timeToLive := computeTimeToLive(response)
	if options.RewriteTTL != nil {
		timeToLive = *options.RewriteTTL
	}
	normalizeTTL(response, timeToLive)
	return timeToLive
}

func (c *Client) backgroundRefreshDNS(transport adapter.DNSTransport, question dns.Question, message *dns.Msg, options adapter.DNSQueryOptions, responseChecker func(response *dns.Msg) bool) {
	key := dnsCacheKey{Question: question, transportTag: transport.Tag()}
	_, loaded := c.backgroundRefresh.LoadOrStore(key, struct{}{})
	if loaded {
		return
	}
	go func() {
		defer c.backgroundRefresh.Delete(key)
		ctx := contextWithTransportTag(c.ctx, transport.Tag())
		response, err := c.exchangeToTransport(ctx, transport, message)
		if err != nil {
			if c.logger != nil {
				c.logger.DebugContext(ctx, "optimistic refresh failed for ", FqdnToDomain(question.Name), ": ", err)
			}
			return
		}
		if responseChecker != nil {
			var rejected bool
			if response.Rcode != dns.RcodeSuccess && response.Rcode != dns.RcodeNameError {
				rejected = true
			} else {
				rejected = !responseChecker(response)
			}
			if rejected {
				if c.logger != nil {
					c.logger.DebugContext(ctx, "optimistic refresh rejected for ", FqdnToDomain(question.Name))
				}
				if c.rdrc != nil {
					c.rdrc.SaveRDRCAsync(transport.Tag(), question.Name, question.Qtype, c.logger)
				}
				return
			}
		} else if response.Rcode != dns.RcodeSuccess && response.Rcode != dns.RcodeNameError {
			return
		}
		timeToLive := applyResponseOptions(question, response, options)
		c.storeCache(transport, question, response, timeToLive)
		logRefreshedResponse(c.logger, ctx, response, timeToLive)
	}()
}

func (c *Client) prepareExchangeMessage(message *dns.Msg, options adapter.DNSQueryOptions) *dns.Msg {
	clientSubnet := options.ClientSubnet
	if !clientSubnet.IsValid() {
		clientSubnet = c.clientSubnet
	}
	if clientSubnet.IsValid() {
		message = SetClientSubnet(message, clientSubnet)
	}
	return message
}

func stripDNSPadding(response *dns.Msg) {
	for _, record := range response.Extra {
		opt, isOpt := record.(*dns.OPT)
		if !isOpt {
			continue
		}
		opt.Option = common.Filter(opt.Option, func(it dns.EDNS0) bool {
			return it.Option() != dns.EDNS0PADDING
		})
	}
}

func (c *Client) exchangeToTransport(ctx context.Context, transport adapter.DNSTransport, message *dns.Msg) (*dns.Msg, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := transport.Exchange(ctx, message)
	if err == nil {
		stripDNSPadding(response)
		return response, nil
	}
	var rcodeError RcodeError
	if errors.As(err, &rcodeError) {
		return FixedResponseStatus(message, int(rcodeError)), nil
	}
	return nil, err
}

func MessageToAddresses(response *dns.Msg) []netip.Addr {
	return adapter.DNSResponseAddresses(response)
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
