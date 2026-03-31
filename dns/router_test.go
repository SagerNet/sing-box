package dns

import (
	"context"
	"io"
	"net"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental/deprecated"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	rulepkg "github.com/sagernet/sing-box/route/rule"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json/badoption"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/service"

	mDNS "github.com/miekg/dns"
	"github.com/stretchr/testify/require"
	"go4.org/netipx"
)

type fakeDNSTransport struct {
	tag           string
	transportType string
}

func (t *fakeDNSTransport) Start(adapter.StartStage) error { return nil }
func (t *fakeDNSTransport) Close() error                   { return nil }
func (t *fakeDNSTransport) Type() string                   { return t.transportType }
func (t *fakeDNSTransport) Tag() string                    { return t.tag }
func (t *fakeDNSTransport) Dependencies() []string         { return nil }
func (t *fakeDNSTransport) Reset()                         {}
func (t *fakeDNSTransport) Exchange(context.Context, *mDNS.Msg) (*mDNS.Msg, error) {
	return nil, E.New("unused transport exchange")
}

type fakeDNSTransportManager struct {
	defaultTransport adapter.DNSTransport
	transports       map[string]adapter.DNSTransport
}

func (m *fakeDNSTransportManager) Start(adapter.StartStage) error { return nil }
func (m *fakeDNSTransportManager) Close() error                   { return nil }
func (m *fakeDNSTransportManager) Transports() []adapter.DNSTransport {
	transports := make([]adapter.DNSTransport, 0, len(m.transports))
	for _, transport := range m.transports {
		transports = append(transports, transport)
	}
	return transports
}

func (m *fakeDNSTransportManager) Transport(tag string) (adapter.DNSTransport, bool) {
	transport, loaded := m.transports[tag]
	return transport, loaded
}
func (m *fakeDNSTransportManager) Default() adapter.DNSTransport { return m.defaultTransport }
func (m *fakeDNSTransportManager) FakeIP() adapter.FakeIPTransport {
	return nil
}
func (m *fakeDNSTransportManager) Remove(string) error { return nil }
func (m *fakeDNSTransportManager) Create(context.Context, log.ContextLogger, string, string, any) error {
	return E.New("unsupported")
}

type fakeDNSClient struct {
	beforeExchange func(ctx context.Context, transport adapter.DNSTransport, message *mDNS.Msg)
	exchange       func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error)
	lookup         func(transport adapter.DNSTransport, domain string, options adapter.DNSQueryOptions) ([]netip.Addr, *mDNS.Msg, error)
}

type fakeDeprecatedManager struct {
	features []deprecated.Note
}

type fakeRouter struct {
	access   sync.RWMutex
	ruleSets map[string]adapter.RuleSet
}

func (r *fakeRouter) Start(adapter.StartStage) error { return nil }
func (r *fakeRouter) Close() error                   { return nil }
func (r *fakeRouter) PreMatch(metadata adapter.InboundContext, _ tun.DirectRouteContext, _ time.Duration, _ bool) (tun.DirectRouteDestination, error) {
	return nil, nil
}

func (r *fakeRouter) RouteConnection(context.Context, net.Conn, adapter.InboundContext) error {
	return nil
}

func (r *fakeRouter) RoutePacketConnection(context.Context, N.PacketConn, adapter.InboundContext) error {
	return nil
}

func (r *fakeRouter) RouteConnectionEx(context.Context, net.Conn, adapter.InboundContext, N.CloseHandlerFunc) {
}

func (r *fakeRouter) RoutePacketConnectionEx(context.Context, N.PacketConn, adapter.InboundContext, N.CloseHandlerFunc) {
}

func (r *fakeRouter) RuleSet(tag string) (adapter.RuleSet, bool) {
	r.access.RLock()
	defer r.access.RUnlock()
	ruleSet, loaded := r.ruleSets[tag]
	return ruleSet, loaded
}

func (r *fakeRouter) setRuleSet(tag string, ruleSet adapter.RuleSet) {
	r.access.Lock()
	defer r.access.Unlock()
	if r.ruleSets == nil {
		r.ruleSets = make(map[string]adapter.RuleSet)
	}
	r.ruleSets[tag] = ruleSet
}
func (r *fakeRouter) Rules() []adapter.Rule                      { return nil }
func (r *fakeRouter) NeedFindProcess() bool                      { return false }
func (r *fakeRouter) NeedFindNeighbor() bool                     { return false }
func (r *fakeRouter) NeighborResolver() adapter.NeighborResolver { return nil }
func (r *fakeRouter) AppendTracker(adapter.ConnectionTracker)    {}
func (r *fakeRouter) ResetNetwork()                              {}

type fakeRuleSet struct {
	access                   sync.Mutex
	metadata                 adapter.RuleSetMetadata
	metadataRead             func(adapter.RuleSetMetadata) adapter.RuleSetMetadata
	match                    func(*adapter.InboundContext) bool
	callbacks                list.List[adapter.RuleSetUpdateCallback]
	refs                     int
	afterIncrementReference  func()
	beforeDecrementReference func()
}

func (s *fakeRuleSet) Name() string                                                  { return "fake-rule-set" }
func (s *fakeRuleSet) StartContext(context.Context, *adapter.HTTPStartContext) error { return nil }
func (s *fakeRuleSet) PostStart() error                                              { return nil }
func (s *fakeRuleSet) Metadata() adapter.RuleSetMetadata {
	s.access.Lock()
	metadata := s.metadata
	metadataRead := s.metadataRead
	s.access.Unlock()
	if metadataRead != nil {
		return metadataRead(metadata)
	}
	return metadata
}
func (s *fakeRuleSet) ExtractIPSet() []*netipx.IPSet { return nil }
func (s *fakeRuleSet) IncRef() {
	s.access.Lock()
	s.refs++
	afterIncrementReference := s.afterIncrementReference
	s.access.Unlock()
	if afterIncrementReference != nil {
		afterIncrementReference()
	}
}

func (s *fakeRuleSet) DecRef() {
	s.access.Lock()
	beforeDecrementReference := s.beforeDecrementReference
	s.access.Unlock()
	if beforeDecrementReference != nil {
		beforeDecrementReference()
	}
	s.access.Lock()
	defer s.access.Unlock()
	s.refs--
	if s.refs < 0 {
		panic("rule-set: negative refs")
	}
}
func (s *fakeRuleSet) Cleanup() {}
func (s *fakeRuleSet) RegisterCallback(callback adapter.RuleSetUpdateCallback) *list.Element[adapter.RuleSetUpdateCallback] {
	s.access.Lock()
	defer s.access.Unlock()
	return s.callbacks.PushBack(callback)
}

func (s *fakeRuleSet) UnregisterCallback(element *list.Element[adapter.RuleSetUpdateCallback]) {
	s.access.Lock()
	defer s.access.Unlock()
	s.callbacks.Remove(element)
}
func (s *fakeRuleSet) Close() error { return nil }
func (s *fakeRuleSet) Match(metadata *adapter.InboundContext) bool {
	s.access.Lock()
	match := s.match
	s.access.Unlock()
	if match != nil {
		return match(metadata)
	}
	return true
}
func (s *fakeRuleSet) String() string { return "fake-rule-set" }
func (s *fakeRuleSet) updateMetadata(metadata adapter.RuleSetMetadata) {
	s.access.Lock()
	s.metadata = metadata
	callbacks := s.callbacks.Array()
	s.access.Unlock()
	for _, callback := range callbacks {
		callback(s)
	}
}

func (s *fakeRuleSet) snapshotCallbacks() []adapter.RuleSetUpdateCallback {
	s.access.Lock()
	defer s.access.Unlock()
	return s.callbacks.Array()
}

func (s *fakeRuleSet) refCount() int {
	s.access.Lock()
	defer s.access.Unlock()
	return s.refs
}

func (m *fakeDeprecatedManager) ReportDeprecated(feature deprecated.Note) {
	m.features = append(m.features, feature)
}

func (c *fakeDNSClient) Start() {}

func (c *fakeDNSClient) Exchange(ctx context.Context, transport adapter.DNSTransport, message *mDNS.Msg, _ adapter.DNSQueryOptions, _ func(*mDNS.Msg) bool) (*mDNS.Msg, error) {
	if c.beforeExchange != nil {
		c.beforeExchange(ctx, transport, message)
	}
	return c.exchange(transport, message)
}

func (c *fakeDNSClient) Lookup(_ context.Context, transport adapter.DNSTransport, domain string, options adapter.DNSQueryOptions, responseChecker func(*mDNS.Msg) bool) ([]netip.Addr, error) {
	if c.lookup == nil {
		return nil, E.New("unused client lookup")
	}
	addresses, response, err := c.lookup(transport, domain, options)
	if err != nil {
		return nil, err
	}
	if response == nil {
		response = FixedResponse(0, fixedQuestion(domain, mDNS.TypeA), addresses, 60)
	}
	if responseChecker != nil && !responseChecker(response) {
		return nil, ErrResponseRejected
	}
	if addresses != nil {
		return addresses, nil
	}
	return MessageToAddresses(response), nil
}

func (c *fakeDNSClient) ClearCache() {}

func newTestRouter(t *testing.T, rules []option.DNSRule, transportManager *fakeDNSTransportManager, client *fakeDNSClient) *Router {
	return newTestRouterWithContext(t, context.Background(), rules, transportManager, client)
}

func newTestRouterWithContext(t *testing.T, ctx context.Context, rules []option.DNSRule, transportManager *fakeDNSTransportManager, client *fakeDNSClient) *Router {
	return newTestRouterWithContextAndLogger(t, ctx, rules, transportManager, client, log.NewNOPFactory().NewLogger("dns"))
}

func newTestRouterWithContextAndLogger(t *testing.T, ctx context.Context, rules []option.DNSRule, transportManager *fakeDNSTransportManager, client *fakeDNSClient, dnsLogger log.ContextLogger) *Router {
	t.Helper()
	router := &Router{
		ctx:                   ctx,
		logger:                dnsLogger,
		transport:             transportManager,
		client:                client,
		rawRules:              make([]option.DNSRule, 0, len(rules)),
		defaultDomainStrategy: C.DomainStrategyAsIS,
	}
	router.currentRules.Store(newRulesSnapshot(make([]adapter.DNSRule, 0, len(rules)), false))
	if rules != nil {
		err := router.Initialize(rules)
		require.NoError(t, err)
		err = router.Start(adapter.StartStateStart)
		require.NoError(t, err)
	}
	return router
}

func waitForLogMessageContaining(t *testing.T, entries <-chan log.Entry, done <-chan struct{}, substring string) log.Entry {
	t.Helper()
	timeout := time.After(time.Second)
	for {
		select {
		case entry, ok := <-entries:
			if !ok {
				t.Fatal("log subscription closed")
			}
			if strings.Contains(entry.Message, substring) {
				return entry
			}
		case <-done:
			t.Fatal("log subscription closed")
		case <-timeout:
			t.Fatalf("timed out waiting for log message containing %q", substring)
		}
	}
}

func fixedQuestion(name string, qType uint16) mDNS.Question {
	return mDNS.Question{
		Name:   mDNS.Fqdn(name),
		Qtype:  qType,
		Qclass: mDNS.ClassINET,
	}
}

func mustRecord(t *testing.T, record string) option.DNSRecordOptions {
	t.Helper()
	var value option.DNSRecordOptions
	require.NoError(t, value.UnmarshalJSON([]byte(`"`+record+`"`)))
	return value
}

func fixedHTTPSHintResponse(question mDNS.Question, addresses ...netip.Addr) *mDNS.Msg {
	response := &mDNS.Msg{
		MsgHdr: mDNS.MsgHdr{
			Response: true,
			Rcode:    mDNS.RcodeSuccess,
		},
		Question: []mDNS.Question{question},
		Answer: []mDNS.RR{
			&mDNS.HTTPS{
				SVCB: mDNS.SVCB{
					Hdr: mDNS.RR_Header{
						Name:   question.Name,
						Rrtype: mDNS.TypeHTTPS,
						Class:  mDNS.ClassINET,
						Ttl:    60,
					},
					Priority: 1,
					Target:   ".",
				},
			},
		},
	}
	https := response.Answer[0].(*mDNS.HTTPS)
	var (
		hints4 []net.IP
		hints6 []net.IP
	)
	for _, address := range addresses {
		if address.Is4() {
			hints4 = append(hints4, net.IP(append([]byte(nil), address.AsSlice()...)))
		} else {
			hints6 = append(hints6, net.IP(append([]byte(nil), address.AsSlice()...)))
		}
	}
	if len(hints4) > 0 {
		https.SVCB.Value = append(https.SVCB.Value, &mDNS.SVCBIPv4Hint{Hint: hints4})
	}
	if len(hints6) > 0 {
		https.SVCB.Value = append(https.SVCB.Value, &mDNS.SVCBIPv6Hint{Hint: hints6})
	}
	return response
}

func fixedHTTPSHintResponseWithRawHints(question mDNS.Question, ipv4Hints []net.IP, ipv6Hints []net.IP) *mDNS.Msg {
	response := fixedHTTPSHintResponse(question)
	https := response.Answer[0].(*mDNS.HTTPS)
	if len(ipv4Hints) > 0 {
		hints := make([]net.IP, 0, len(ipv4Hints))
		for _, ip := range ipv4Hints {
			hints = append(hints, net.IP(append([]byte(nil), ip...)))
		}
		https.SVCB.Value = append(https.SVCB.Value, &mDNS.SVCBIPv4Hint{Hint: hints})
	}
	if len(ipv6Hints) > 0 {
		hints := make([]net.IP, 0, len(ipv6Hints))
		for _, ip := range ipv6Hints {
			hints = append(hints, net.IP(append([]byte(nil), ip...)))
		}
		https.SVCB.Value = append(https.SVCB.Value, &mDNS.SVCBIPv6Hint{Hint: hints})
	}
	return response
}

func TestValidateLegacyDNSModeDisabledRules_RequireMatchResponseForDirectIPCIDR(t *testing.T) {
	t.Parallel()

	err := validateLegacyDNSModeDisabledRules([]option.DNSRule{{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				IPCIDR: badoption.Listable[string]{"1.1.1.0/24"},
			},
			DNSRuleAction: option.DNSRuleAction{
				Action: C.RuleActionTypeRoute,
				RouteOptions: option.DNSRouteActionOptions{
					Server: "default",
				},
			},
		},
	}})
	require.ErrorContains(t, err, "ip_cidr and ip_is_private require match_response")
}

func TestValidateLegacyDNSModeDisabledRules_AllowMatchResponseWithoutEvaluate(t *testing.T) {
	t.Parallel()

	err := validateLegacyDNSModeDisabledRules([]option.DNSRule{{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				MatchResponse: true,
				IPCIDR:        badoption.Listable[string]{"1.1.1.0/24"},
			},
		},
	}})
	require.NoError(t, err)
}

func TestInitializeRejectsInvalidDNSRuleParseError(t *testing.T) {
	t.Parallel()

	router := &Router{
		ctx:                   context.Background(),
		logger:                log.NewNOPFactory().NewLogger("dns"),
		transport:             &fakeDNSTransportManager{},
		client:                &fakeDNSClient{},
		rawRules:              make([]option.DNSRule, 0, 1),
		defaultDomainStrategy: C.DomainStrategyAsIS,
	}
	router.currentRules.Store(newRulesSnapshot(make([]adapter.DNSRule, 0, 1), false))
	err := router.Initialize([]option.DNSRule{{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				DomainRegex: badoption.Listable[string]{"("},
			},
			DNSRuleAction: option.DNSRuleAction{
				Action:       C.RuleActionTypeRoute,
				RouteOptions: option.DNSRouteActionOptions{Server: "default"},
			},
		},
	}})
	require.ErrorContains(t, err, "domain_regex")
}

func TestInitializeRejectsDirectLegacyRuleWhenRuleSetForcesNew(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ruleSet, err := rulepkg.NewRuleSet(ctx, log.NewNOPFactory().NewLogger("router"), option.RuleSet{
		Type: C.RuleSetTypeInline,
		Tag:  "query-set",
		InlineOptions: option.PlainRuleSet{
			Rules: []option.HeadlessRule{{
				Type: C.RuleTypeDefault,
				DefaultOptions: option.DefaultHeadlessRule{
					QueryType: badoption.Listable[option.DNSQueryType]{option.DNSQueryType(mDNS.TypeA)},
				},
			}},
		},
	})
	require.NoError(t, err)
	ctx = service.ContextWith[adapter.Router](ctx, &fakeRouter{
		ruleSets: map[string]adapter.RuleSet{
			"query-set": ruleSet,
		},
	})

	router := &Router{
		ctx:                   ctx,
		logger:                log.NewNOPFactory().NewLogger("dns"),
		transport:             &fakeDNSTransportManager{},
		client:                &fakeDNSClient{},
		rawRules:              make([]option.DNSRule, 0, 2),
		defaultDomainStrategy: C.DomainStrategyAsIS,
	}
	router.currentRules.Store(newRulesSnapshot(make([]adapter.DNSRule, 0, 2), false))
	err = router.Initialize([]option.DNSRule{
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					RuleSet: badoption.Listable[string]{"query-set"},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "default"},
				},
			},
		},
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					IPIsPrivate: true,
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "private"},
				},
			},
		},
	})
	require.ErrorContains(t, err, "ip_cidr and ip_is_private require match_response")
}

func TestLookupLegacyDNSModeDefersRuleSetDestinationIPMatch(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ruleSet, err := rulepkg.NewRuleSet(ctx, log.NewNOPFactory().NewLogger("router"), option.RuleSet{
		Type: C.RuleSetTypeInline,
		Tag:  "legacy-ipcidr-set",
		InlineOptions: option.PlainRuleSet{
			Rules: []option.HeadlessRule{{
				Type: C.RuleTypeDefault,
				DefaultOptions: option.DefaultHeadlessRule{
					IPCIDR: badoption.Listable[string]{"10.0.0.0/8"},
				},
			}},
		},
	})
	require.NoError(t, err)
	ctx = service.ContextWith[adapter.Router](ctx, &fakeRouter{
		ruleSets: map[string]adapter.RuleSet{
			"legacy-ipcidr-set": ruleSet,
		},
	})

	defaultTransport := &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP}
	privateTransport := &fakeDNSTransport{tag: "private", transportType: C.DNSTypeUDP}
	router := newTestRouterWithContext(t, ctx, []option.DNSRule{{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				RuleSet: badoption.Listable[string]{"legacy-ipcidr-set"},
			},
			DNSRuleAction: option.DNSRuleAction{
				Action:       C.RuleActionTypeRoute,
				RouteOptions: option.DNSRouteActionOptions{Server: "private"},
			},
		},
	}}, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default": defaultTransport,
			"private": privateTransport,
		},
	}, &fakeDNSClient{
		lookup: func(transport adapter.DNSTransport, domain string, options adapter.DNSQueryOptions) ([]netip.Addr, *mDNS.Msg, error) {
			require.Equal(t, "example.com", domain)
			require.Equal(t, "private", transport.Tag())
			response := FixedResponse(0, fixedQuestion(domain, mDNS.TypeA), []netip.Addr{netip.MustParseAddr("10.0.0.1")}, 60)
			return MessageToAddresses(response), response, nil
		},
	})

	require.True(t, router.currentRules.Load().legacyDNSMode)

	addresses, err := router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{
		LookupStrategy: C.DomainStrategyIPv4Only,
	})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("10.0.0.1")}, addresses)
}

func TestRuleSetUpdateReleasesOldRuleSetRefs(t *testing.T) {
	t.Parallel()

	fakeSet := &fakeRuleSet{}
	ctx := service.ContextWith[adapter.Router](context.Background(), &fakeRouter{
		ruleSets: map[string]adapter.RuleSet{
			"dynamic-set": fakeSet,
		},
	})
	defaultTransport := &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP}
	router := newTestRouterWithContext(t, ctx, []option.DNSRule{{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				RuleSet: badoption.Listable[string]{"dynamic-set"},
			},
			DNSRuleAction: option.DNSRuleAction{
				Action:       C.RuleActionTypeRoute,
				RouteOptions: option.DNSRouteActionOptions{Server: "default"},
			},
		},
	}}, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default": defaultTransport,
		},
	}, &fakeDNSClient{})

	require.Equal(t, 1, fakeSet.refCount())

	fakeSet.updateMetadata(adapter.RuleSetMetadata{})
	require.Equal(t, 1, fakeSet.refCount())

	fakeSet.updateMetadata(adapter.RuleSetMetadata{})
	require.Equal(t, 1, fakeSet.refCount())

	require.NoError(t, router.Close())
	require.Zero(t, fakeSet.refCount())
}

func TestRuleSetUpdateKeepsLastSuccessfullyCompiledRuleGraphWhenRebuildFails(t *testing.T) {
	t.Parallel()

	callbackRuleSet := &fakeRuleSet{
		match: func(*adapter.InboundContext) bool {
			return false
		},
	}
	routerService := &fakeRouter{
		ruleSets: map[string]adapter.RuleSet{
			"dynamic-set": callbackRuleSet,
		},
	}
	ctx := service.ContextWith[adapter.Router](context.Background(), routerService)
	defaultTransport := &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP}
	preservedTransport := &fakeDNSTransport{tag: "preserved", transportType: C.DNSTypeUDP}
	wouldBeNewTransport := &fakeDNSTransport{tag: "would-be-new", transportType: C.DNSTypeUDP}
	loggerFactory := log.NewDefaultFactory(
		context.Background(),
		log.Formatter{
			BaseTime:         time.Now(),
			DisableColors:    true,
			DisableTimestamp: true,
		},
		io.Discard,
		"",
		nil,
		true,
	)
	loggerFactory.SetLevel(log.LevelError)
	logEntries, logDone, err := loggerFactory.Subscribe()
	require.NoError(t, err)
	t.Cleanup(func() {
		loggerFactory.UnSubscribe(logEntries)
		closeErr := loggerFactory.Close()
		require.NoError(t, closeErr)
	})
	var lastUsedTransport common.TypedValue[string]
	router := newTestRouterWithContextAndLogger(t, ctx, []option.DNSRule{
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					RuleSet: badoption.Listable[string]{"dynamic-set"},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "would-be-new"},
				},
			},
		},
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					Domain: badoption.Listable[string]{"example.com"},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "preserved"},
				},
			},
		},
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					IPIsPrivate: true,
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "preserved"},
				},
			},
		},
	}, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default":      defaultTransport,
			"preserved":    preservedTransport,
			"would-be-new": wouldBeNewTransport,
		},
	}, &fakeDNSClient{
		lookup: func(transport adapter.DNSTransport, domain string, options adapter.DNSQueryOptions) ([]netip.Addr, *mDNS.Msg, error) {
			lastUsedTransport.Store(transport.Tag())
			response := FixedResponse(0, fixedQuestion(domain, mDNS.TypeA), []netip.Addr{netip.MustParseAddr("10.0.0.1")}, 60)
			return MessageToAddresses(response), response, nil
		},
	}, loggerFactory.NewLogger("dns"))
	t.Cleanup(func() {
		closeErr := router.Close()
		require.NoError(t, closeErr)
	})

	require.True(t, router.currentRules.Load().legacyDNSMode)
	require.Equal(t, 1, callbackRuleSet.refCount())

	addresses, err := router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("10.0.0.1")}, addresses)
	require.Equal(t, "preserved", lastUsedTransport.Load())

	rebuildTargetRuleSet := &fakeRuleSet{
		metadata: adapter.RuleSetMetadata{
			ContainsDNSQueryTypeRule: true,
		},
		match: func(*adapter.InboundContext) bool {
			return true
		},
	}
	routerService.setRuleSet("dynamic-set", rebuildTargetRuleSet)

	callbackRuleSet.updateMetadata(adapter.RuleSetMetadata{
		ContainsDNSQueryTypeRule: true,
	})
	rebuildErrorEntry := waitForLogMessageContaining(t, logEntries, logDone, "rebuild DNS rules after rule-set update")
	require.Contains(t, rebuildErrorEntry.Message, "ip_cidr and ip_is_private require match_response")
	require.True(t, router.currentRules.Load().legacyDNSMode)
	require.Equal(t, 1, callbackRuleSet.refCount())
	require.Zero(t, rebuildTargetRuleSet.refCount())

	lastUsedTransport.Store("")
	addresses, err = router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("10.0.0.1")}, addresses)
	require.Equal(t, "preserved", lastUsedTransport.Load())
	require.NotEqual(t, "would-be-new", lastUsedTransport.Load())
}

func TestRuleSetUpdateSerializesConcurrentRebuilds(t *testing.T) {
	t.Parallel()

	callbackRuleSet := &fakeRuleSet{
		match: func(*adapter.InboundContext) bool {
			return false
		},
	}
	routerService := &fakeRouter{
		ruleSets: map[string]adapter.RuleSet{
			"dynamic-set": callbackRuleSet,
		},
	}
	ctx := service.ContextWith[adapter.Router](context.Background(), routerService)
	defaultTransport := &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP}
	firstTransport := &fakeDNSTransport{tag: "first", transportType: C.DNSTypeUDP}
	secondTransport := &fakeDNSTransport{tag: "second", transportType: C.DNSTypeUDP}
	var lastUsedTransport common.TypedValue[string]
	router := newTestRouterWithContext(t, ctx, []option.DNSRule{
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					RuleSet: badoption.Listable[string]{"dynamic-set"},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "first"},
				},
			},
		},
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					Domain: badoption.Listable[string]{"example.com"},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "second"},
				},
			},
		},
	}, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default": defaultTransport,
			"first":   firstTransport,
			"second":  secondTransport,
		},
	}, &fakeDNSClient{
		exchange: func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
			lastUsedTransport.Store(transport.Tag())
			return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("10.0.0.1")}, 60), nil
		},
	})

	addresses, err := router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("10.0.0.1")}, addresses)
	require.Equal(t, "second", lastUsedTransport.Load())

	callbacks := callbackRuleSet.snapshotCallbacks()
	require.Len(t, callbacks, 1)

	firstMetadataEntered := make(chan struct{})
	releaseFirstMetadata := make(chan struct{})
	firstRuleSetStarted := make(chan struct{})
	releaseFirstRuleSetStart := make(chan struct{})
	secondMetadataEntered := make(chan struct{})
	releaseSecondMetadata := make(chan struct{})

	var metadataAccess sync.Mutex
	var metadataCallCount int
	var concurrentMetadataCalls int
	var maximumConcurrentMetadataCalls int

	recordMetadataEntry := func() func() {
		metadataAccess.Lock()
		metadataCallCount++
		concurrentMetadataCalls++
		if concurrentMetadataCalls > maximumConcurrentMetadataCalls {
			maximumConcurrentMetadataCalls = concurrentMetadataCalls
		}
		metadataAccess.Unlock()
		return func() {
			metadataAccess.Lock()
			concurrentMetadataCalls--
			metadataAccess.Unlock()
		}
	}

	firstBuildRuleSet := &fakeRuleSet{
		match: func(*adapter.InboundContext) bool {
			return true
		},
		metadataRead: func(metadata adapter.RuleSetMetadata) adapter.RuleSetMetadata {
			metadataDone := recordMetadataEntry()
			close(firstMetadataEntered)
			<-releaseFirstMetadata
			metadataDone()
			return metadata
		},
		afterIncrementReference: func() {
			close(firstRuleSetStarted)
			<-releaseFirstRuleSetStart
		},
	}
	secondBuildRuleSet := &fakeRuleSet{
		match: func(*adapter.InboundContext) bool {
			return false
		},
		metadataRead: func(metadata adapter.RuleSetMetadata) adapter.RuleSetMetadata {
			metadataDone := recordMetadataEntry()
			close(secondMetadataEntered)
			<-releaseSecondMetadata
			metadataDone()
			return metadata
		},
	}

	routerService.setRuleSet("dynamic-set", firstBuildRuleSet)

	firstCallbackFinished := make(chan struct{})
	go func() {
		callbacks[0](callbackRuleSet)
		close(firstCallbackFinished)
	}()

	select {
	case <-firstMetadataEntered:
	case <-time.After(time.Second):
		t.Fatal("first rebuild did not reach rule-set metadata")
	}

	close(releaseFirstMetadata)

	select {
	case <-firstRuleSetStarted:
	case <-time.After(time.Second):
		t.Fatal("first rebuild did not reach rule-set start")
	}

	routerService.setRuleSet("dynamic-set", secondBuildRuleSet)

	secondCallbackStarted := make(chan struct{})
	secondCallbackFinished := make(chan struct{})
	go func() {
		close(secondCallbackStarted)
		callbacks[0](callbackRuleSet)
		close(secondCallbackFinished)
	}()

	select {
	case <-secondCallbackStarted:
	case <-time.After(time.Second):
		t.Fatal("second rebuild did not start")
	}

	select {
	case <-secondMetadataEntered:
		t.Fatal("second rebuild entered rule-set metadata before the first rebuild completed")
	default:
	}

	close(releaseFirstRuleSetStart)

	select {
	case <-firstCallbackFinished:
	case <-time.After(time.Second):
		t.Fatal("first rebuild callback did not finish")
	}

	select {
	case <-secondMetadataEntered:
	case <-time.After(time.Second):
		t.Fatal("second rebuild did not enter rule-set metadata after the first rebuild finished")
	}

	addresses, err = router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("10.0.0.1")}, addresses)
	require.Equal(t, "first", lastUsedTransport.Load())

	close(releaseSecondMetadata)

	select {
	case <-secondCallbackFinished:
	case <-time.After(time.Second):
		t.Fatal("second rebuild callback did not finish")
	}

	metadataAccess.Lock()
	require.Equal(t, 2, metadataCallCount)
	require.Equal(t, 1, maximumConcurrentMetadataCalls)
	metadataAccess.Unlock()
	require.Zero(t, callbackRuleSet.refCount())
	require.Zero(t, firstBuildRuleSet.refCount())
	require.Equal(t, 1, secondBuildRuleSet.refCount())

	lastUsedTransport.Store("")
	addresses, err = router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("10.0.0.1")}, addresses)
	require.Equal(t, "second", lastUsedTransport.Load())

	err = router.Close()
	require.NoError(t, err)
	require.Zero(t, callbackRuleSet.refCount())
	require.Zero(t, firstBuildRuleSet.refCount())
	require.Zero(t, secondBuildRuleSet.refCount())
}

func TestCloseDuringRebuildDiscardsResult(t *testing.T) {
	t.Parallel()

	fakeSet := &fakeRuleSet{
		metadata: adapter.RuleSetMetadata{
			ContainsIPCIDRRule: true,
		},
	}
	ctx := service.ContextWith[adapter.Router](context.Background(), &fakeRouter{
		ruleSets: map[string]adapter.RuleSet{
			"dynamic-set": fakeSet,
		},
	})
	defaultTransport := &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP}
	router := newTestRouterWithContext(t, ctx, []option.DNSRule{
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					RuleSet: badoption.Listable[string]{"dynamic-set"},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "installed"},
				},
			},
		},
	}, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default":   defaultTransport,
			"discarded": &fakeDNSTransport{tag: "discarded", transportType: C.DNSTypeUDP},
			"installed": &fakeDNSTransport{tag: "installed", transportType: C.DNSTypeUDP},
		},
	}, &fakeDNSClient{
		exchange: func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
			switch transport.Tag() {
			case "discarded", "installed", "default":
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("10.0.0.1")}, 60), nil
			default:
				return nil, E.New("unexpected transport: ", transport.Tag())
			}
		},
	})
	require.True(t, router.currentRules.Load().legacyDNSMode)
	require.Equal(t, 1, fakeSet.refCount())

	callbacks := fakeSet.snapshotCallbacks()
	require.Len(t, callbacks, 1)

	firstMetadataEntered := make(chan struct{})
	releaseFirstMetadata := make(chan struct{})
	callbackFinished := make(chan struct{})
	fakeSet.metadataRead = func(metadata adapter.RuleSetMetadata) adapter.RuleSetMetadata {
		router.rawRules[0].DefaultOptions.RouteOptions.Server = "discarded"
		close(firstMetadataEntered)
		<-releaseFirstMetadata
		return adapter.RuleSetMetadata{}
	}

	go func() {
		callbacks[0](fakeSet)
		close(callbackFinished)
	}()

	select {
	case <-firstMetadataEntered:
	case <-time.After(time.Second):
		t.Fatal("rebuild did not reach rule-set metadata")
	}

	err := router.Close()
	require.NoError(t, err)
	close(releaseFirstMetadata)

	select {
	case <-callbackFinished:
	case <-time.After(time.Second):
		t.Fatal("rebuild callback did not finish after close")
	}

	fakeSet.metadataRead = nil

	router.stateAccess.Lock()
	require.True(t, router.closing)
	require.Empty(t, router.ruleSetCallbacks)
	router.stateAccess.Unlock()
	require.Nil(t, router.currentRules.Load())
	require.Zero(t, fakeSet.refCount())
}

func TestCloseIgnoresSnapshottedRuleSetCallback(t *testing.T) {
	t.Parallel()

	fakeSet := &fakeRuleSet{}
	ctx := service.ContextWith[adapter.Router](context.Background(), &fakeRouter{
		ruleSets: map[string]adapter.RuleSet{
			"dynamic-set": fakeSet,
		},
	})
	defaultTransport := &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP}
	router := newTestRouterWithContext(t, ctx, []option.DNSRule{
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					RuleSet: badoption.Listable[string]{"dynamic-set"},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "default"},
				},
			},
		},
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					IPIsPrivate: true,
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "default"},
				},
			},
		},
	}, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default": defaultTransport,
		},
	}, &fakeDNSClient{
		lookup: func(transport adapter.DNSTransport, domain string, options adapter.DNSQueryOptions) ([]netip.Addr, *mDNS.Msg, error) {
			response := FixedResponse(0, fixedQuestion(domain, mDNS.TypeA), []netip.Addr{netip.MustParseAddr("10.0.0.1")}, 60)
			return MessageToAddresses(response), response, nil
		},
	})

	callbacks := fakeSet.snapshotCallbacks()
	require.Len(t, callbacks, 1)

	require.NoError(t, router.Close())
	require.Empty(t, fakeSet.snapshotCallbacks())

	fakeSet.metadata = adapter.RuleSetMetadata{
		ContainsDNSQueryTypeRule: true,
	}
	callbacks[0](fakeSet)

	router.stateAccess.Lock()
	require.True(t, router.closing)
	require.Empty(t, router.ruleSetCallbacks)
	router.stateAccess.Unlock()
	require.Nil(t, router.currentRules.Load())
}

func TestRuleSetUpdateDoesNotBlockOnInFlightLookup(t *testing.T) {
	t.Parallel()

	fakeSet := &fakeRuleSet{
		metadata: adapter.RuleSetMetadata{
			ContainsIPCIDRRule: true,
		},
	}
	ctx := service.ContextWith[adapter.Router](context.Background(), &fakeRouter{
		ruleSets: map[string]adapter.RuleSet{
			"dynamic-set": fakeSet,
		},
	})
	defaultTransport := &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP}
	selectedTransport := &fakeDNSTransport{tag: "selected", transportType: C.DNSTypeUDP}
	lookupStarted := make(chan struct{})
	releaseLookup := make(chan struct{})
	router := newTestRouterWithContext(t, ctx, []option.DNSRule{{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				RuleSet: badoption.Listable[string]{"dynamic-set"},
			},
			DNSRuleAction: option.DNSRuleAction{
				Action:       C.RuleActionTypeRoute,
				RouteOptions: option.DNSRouteActionOptions{Server: "selected"},
			},
		},
	}}, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default":  defaultTransport,
			"selected": selectedTransport,
		},
	}, &fakeDNSClient{
		lookup: func(transport adapter.DNSTransport, domain string, options adapter.DNSQueryOptions) ([]netip.Addr, *mDNS.Msg, error) {
			require.Equal(t, "selected", transport.Tag())
			require.Equal(t, "example.com", domain)
			require.Equal(t, C.DomainStrategyIPv4Only, options.LookupStrategy)
			close(lookupStarted)
			<-releaseLookup
			response := FixedResponse(0, fixedQuestion(domain, mDNS.TypeA), []netip.Addr{netip.MustParseAddr("10.0.0.1")}, 60)
			return MessageToAddresses(response), response, nil
		},
	})
	t.Cleanup(func() {
		closeErr := router.Close()
		require.NoError(t, closeErr)
	})

	require.True(t, router.currentRules.Load().legacyDNSMode)
	require.Equal(t, 1, fakeSet.refCount())

	var (
		addresses []netip.Addr
		err       error
	)
	lookupDone := make(chan struct{})
	go func() {
		addresses, err = router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{
			LookupStrategy: C.DomainStrategyIPv4Only,
		})
		close(lookupDone)
	}()

	select {
	case <-lookupStarted:
	case <-time.After(time.Second):
		t.Fatal("lookup did not reach DNS client")
	}

	rebuildDone := make(chan struct{})
	go func() {
		fakeSet.updateMetadata(adapter.RuleSetMetadata{
			ContainsIPCIDRRule: true,
		})
		close(rebuildDone)
	}()

	select {
	case <-rebuildDone:
	case <-time.After(time.Second):
		t.Fatal("rebuild blocked on in-flight lookup")
	}

	require.Equal(t, 2, fakeSet.refCount())

	select {
	case <-lookupDone:
		t.Fatal("lookup finished before release")
	default:
	}

	close(releaseLookup)

	select {
	case <-lookupDone:
	case <-time.After(time.Second):
		t.Fatal("lookup did not finish after release")
	}

	require.NoError(t, err)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("10.0.0.1")}, addresses)
	require.Eventually(t, func() bool {
		return fakeSet.refCount() == 1
	}, time.Second, 10*time.Millisecond)
}

func TestCloseReleasesSnapshottedRulesAfterInFlightLookup(t *testing.T) {
	t.Parallel()

	fakeSet := &fakeRuleSet{
		metadata: adapter.RuleSetMetadata{
			ContainsIPCIDRRule: true,
		},
	}
	ctx := service.ContextWith[adapter.Router](context.Background(), &fakeRouter{
		ruleSets: map[string]adapter.RuleSet{
			"dynamic-set": fakeSet,
		},
	})
	defaultTransport := &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP}
	selectedTransport := &fakeDNSTransport{tag: "selected", transportType: C.DNSTypeUDP}
	lookupStarted := make(chan struct{})
	releaseLookup := make(chan struct{})
	router := newTestRouterWithContext(t, ctx, []option.DNSRule{{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				RuleSet: badoption.Listable[string]{"dynamic-set"},
			},
			DNSRuleAction: option.DNSRuleAction{
				Action:       C.RuleActionTypeRoute,
				RouteOptions: option.DNSRouteActionOptions{Server: "selected"},
			},
		},
	}}, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default":  defaultTransport,
			"selected": selectedTransport,
		},
	}, &fakeDNSClient{
		lookup: func(transport adapter.DNSTransport, domain string, options adapter.DNSQueryOptions) ([]netip.Addr, *mDNS.Msg, error) {
			require.Equal(t, "selected", transport.Tag())
			require.Equal(t, "example.com", domain)
			require.Equal(t, C.DomainStrategyIPv4Only, options.LookupStrategy)
			close(lookupStarted)
			<-releaseLookup
			response := FixedResponse(0, fixedQuestion(domain, mDNS.TypeA), []netip.Addr{netip.MustParseAddr("10.0.0.1")}, 60)
			return MessageToAddresses(response), response, nil
		},
	})

	require.True(t, router.currentRules.Load().legacyDNSMode)
	require.Equal(t, 1, fakeSet.refCount())

	var (
		addresses []netip.Addr
		lookupErr error
		closeErr  error
	)
	lookupDone := make(chan struct{})
	go func() {
		addresses, lookupErr = router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{
			LookupStrategy: C.DomainStrategyIPv4Only,
		})
		close(lookupDone)
	}()

	select {
	case <-lookupStarted:
	case <-time.After(time.Second):
		t.Fatal("lookup did not reach DNS client")
	}

	closeDone := make(chan struct{})
	go func() {
		closeErr = router.Close()
		close(closeDone)
	}()

	require.Eventually(t, func() bool {
		return router.currentRules.Load() == nil && fakeSet.refCount() == 1
	}, time.Second, 10*time.Millisecond)

	close(releaseLookup)

	select {
	case <-lookupDone:
	case <-time.After(time.Second):
		t.Fatal("lookup did not finish after release")
	}
	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatal("close did not finish")
	}

	require.NoError(t, lookupErr)
	require.NoError(t, closeErr)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("10.0.0.1")}, addresses)
	require.Eventually(t, func() bool {
		return fakeSet.refCount() == 0
	}, time.Second, 10*time.Millisecond)
}

func TestLookupLegacyDNSModeDefersDirectDestinationIPMatch(t *testing.T) {
	t.Parallel()

	defaultTransport := &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP}
	privateTransport := &fakeDNSTransport{tag: "private", transportType: C.DNSTypeUDP}
	client := &fakeDNSClient{
		lookup: func(transport adapter.DNSTransport, domain string, options adapter.DNSQueryOptions) ([]netip.Addr, *mDNS.Msg, error) {
			require.Equal(t, "example.com", domain)
			require.Equal(t, C.DomainStrategyIPv4Only, options.LookupStrategy)
			switch transport.Tag() {
			case "private":
				response := FixedResponse(0, fixedQuestion(domain, mDNS.TypeA), []netip.Addr{netip.MustParseAddr("10.0.0.1")}, 60)
				return MessageToAddresses(response), response, nil
			case "default":
				t.Fatal("default transport should not be used when legacy rule matches after response")
			}
			return nil, nil, E.New("unexpected transport")
		},
	}
	router := newTestRouter(t, []option.DNSRule{{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				IPIsPrivate: true,
			},
			DNSRuleAction: option.DNSRuleAction{
				Action:       C.RuleActionTypeRoute,
				RouteOptions: option.DNSRouteActionOptions{Server: "private"},
			},
		},
	}}, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default": defaultTransport,
			"private": privateTransport,
		},
	}, client)

	require.True(t, router.currentRules.Load().legacyDNSMode)

	addresses, err := router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{
		LookupStrategy: C.DomainStrategyIPv4Only,
	})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("10.0.0.1")}, addresses)
}

func TestLookupLegacyDNSModeFallsBackAfterRejectedAddressLimitResponse(t *testing.T) {
	t.Parallel()

	defaultTransport := &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP}
	privateTransport := &fakeDNSTransport{tag: "private", transportType: C.DNSTypeUDP}
	var lookupAccess sync.Mutex
	var lookupTags []string
	recordLookup := func(tag string) {
		lookupAccess.Lock()
		lookupTags = append(lookupTags, tag)
		lookupAccess.Unlock()
	}
	currentLookupTags := func() []string {
		lookupAccess.Lock()
		defer lookupAccess.Unlock()
		return append([]string(nil), lookupTags...)
	}
	client := &fakeDNSClient{
		lookup: func(transport adapter.DNSTransport, domain string, options adapter.DNSQueryOptions) ([]netip.Addr, *mDNS.Msg, error) {
			require.Equal(t, "example.com", domain)
			require.Equal(t, C.DomainStrategyIPv4Only, options.LookupStrategy)
			recordLookup(transport.Tag())
			switch transport.Tag() {
			case "private":
				response := FixedResponse(0, fixedQuestion(domain, mDNS.TypeA), []netip.Addr{netip.MustParseAddr("8.8.8.8")}, 60)
				return MessageToAddresses(response), response, nil
			case "default":
				response := FixedResponse(0, fixedQuestion(domain, mDNS.TypeA), []netip.Addr{netip.MustParseAddr("9.9.9.9")}, 60)
				return MessageToAddresses(response), response, nil
			}
			return nil, nil, E.New("unexpected transport")
		},
	}
	router := newTestRouter(t, []option.DNSRule{{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				IPIsPrivate: true,
			},
			DNSRuleAction: option.DNSRuleAction{
				Action:       C.RuleActionTypeRoute,
				RouteOptions: option.DNSRouteActionOptions{Server: "private"},
			},
		},
	}}, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default": defaultTransport,
			"private": privateTransport,
		},
	}, client)

	addresses, err := router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{
		LookupStrategy: C.DomainStrategyIPv4Only,
	})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("9.9.9.9")}, addresses)
	require.Equal(t, []string{"private", "default"}, currentLookupTags())
}

func TestLookupLegacyDNSModeRuleSetAcceptEmptyDoesNotTreatMismatchAsEmpty(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ruleSet, err := rulepkg.NewRuleSet(ctx, log.NewNOPFactory().NewLogger("router"), option.RuleSet{
		Type: C.RuleSetTypeInline,
		Tag:  "legacy-ipcidr-set",
		InlineOptions: option.PlainRuleSet{
			Rules: []option.HeadlessRule{{
				Type: C.RuleTypeDefault,
				DefaultOptions: option.DefaultHeadlessRule{
					IPCIDR: badoption.Listable[string]{"10.0.0.0/8"},
				},
			}},
		},
	})
	require.NoError(t, err)
	ctx = service.ContextWith[adapter.Router](ctx, &fakeRouter{
		ruleSets: map[string]adapter.RuleSet{
			"legacy-ipcidr-set": ruleSet,
		},
	})

	defaultTransport := &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP}
	privateTransport := &fakeDNSTransport{tag: "private", transportType: C.DNSTypeUDP}
	var lookupAccess sync.Mutex
	var lookupTags []string
	recordLookup := func(tag string) {
		lookupAccess.Lock()
		lookupTags = append(lookupTags, tag)
		lookupAccess.Unlock()
	}
	currentLookupTags := func() []string {
		lookupAccess.Lock()
		defer lookupAccess.Unlock()
		return append([]string(nil), lookupTags...)
	}
	router := newTestRouterWithContext(t, ctx, []option.DNSRule{
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					RuleSet:                  badoption.Listable[string]{"legacy-ipcidr-set"},
					RuleSetIPCIDRAcceptEmpty: true,
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "private"},
				},
			},
		},
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "default"},
				},
			},
		},
	}, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default": defaultTransport,
			"private": privateTransport,
		},
	}, &fakeDNSClient{
		lookup: func(transport adapter.DNSTransport, domain string, options adapter.DNSQueryOptions) ([]netip.Addr, *mDNS.Msg, error) {
			require.Equal(t, "example.com", domain)
			require.Equal(t, C.DomainStrategyIPv4Only, options.LookupStrategy)
			recordLookup(transport.Tag())
			switch transport.Tag() {
			case "private":
				response := FixedResponse(0, fixedQuestion(domain, mDNS.TypeA), []netip.Addr{netip.MustParseAddr("8.8.8.8")}, 60)
				return MessageToAddresses(response), response, nil
			case "default":
				response := FixedResponse(0, fixedQuestion(domain, mDNS.TypeA), []netip.Addr{netip.MustParseAddr("9.9.9.9")}, 60)
				return MessageToAddresses(response), response, nil
			}
			return nil, nil, E.New("unexpected transport")
		},
	})

	require.True(t, router.currentRules.Load().legacyDNSMode)

	addresses, err := router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{
		LookupStrategy: C.DomainStrategyIPv4Only,
	})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("9.9.9.9")}, addresses)
	require.Equal(t, []string{"private", "default"}, currentLookupTags())
}

func TestDNSResponseAddressesMatchesMessageToAddressesForHTTPSHints(t *testing.T) {
	t.Parallel()

	response := fixedHTTPSHintResponse(fixedQuestion("example.com", mDNS.TypeHTTPS),
		netip.MustParseAddr("1.1.1.1"),
		netip.MustParseAddr("2001:db8::1"),
	)

	require.Equal(t, MessageToAddresses(response), adapter.DNSResponseAddresses(response))
}

func TestExchangeLegacyDNSModeDisabledEvaluateMatchResponseRoute(t *testing.T) {
	t.Parallel()

	transportManager := &fakeDNSTransportManager{
		defaultTransport: &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		transports: map[string]adapter.DNSTransport{
			"upstream": &fakeDNSTransport{tag: "upstream", transportType: C.DNSTypeUDP},
			"selected": &fakeDNSTransport{tag: "selected", transportType: C.DNSTypeUDP},
			"default":  &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		},
	}
	client := &fakeDNSClient{
		exchange: func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
			switch transport.Tag() {
			case "upstream":
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("1.1.1.1")}, 60), nil
			case "selected":
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("8.8.8.8")}, 60), nil
			default:
				return nil, E.New("unexpected transport")
			}
		},
	}
	rules := []option.DNSRule{
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					Domain: badoption.Listable[string]{"example.com"},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeEvaluate,
					RouteOptions: option.DNSRouteActionOptions{Server: "upstream"},
				},
			},
		},
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					MatchResponse:  true,
					ResponseAnswer: badoption.Listable[option.DNSRecordOptions]{mustRecord(t, "example.com. IN A 1.1.1.1")},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "selected"},
				},
			},
		},
	}
	router := newTestRouter(t, rules, transportManager, client)

	response, err := router.Exchange(context.Background(), &mDNS.Msg{
		Question: []mDNS.Question{fixedQuestion("example.com", mDNS.TypeA)},
	}, adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("8.8.8.8")}, MessageToAddresses(response))
}

func TestExchangeLegacyDNSModeDisabledEvaluateMatchResponseRouteIgnoresTTL(t *testing.T) {
	t.Parallel()

	transportManager := &fakeDNSTransportManager{
		defaultTransport: &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		transports: map[string]adapter.DNSTransport{
			"upstream": &fakeDNSTransport{tag: "upstream", transportType: C.DNSTypeUDP},
			"selected": &fakeDNSTransport{tag: "selected", transportType: C.DNSTypeUDP},
			"default":  &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		},
	}
	client := &fakeDNSClient{
		exchange: func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
			switch transport.Tag() {
			case "upstream":
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("1.1.1.1")}, 30), nil
			case "selected":
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("8.8.8.8")}, 60), nil
			default:
				return nil, E.New("unexpected transport")
			}
		},
	}
	rules := []option.DNSRule{
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					Domain: badoption.Listable[string]{"example.com"},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeEvaluate,
					RouteOptions: option.DNSRouteActionOptions{Server: "upstream"},
				},
			},
		},
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					MatchResponse:  true,
					ResponseAnswer: badoption.Listable[option.DNSRecordOptions]{mustRecord(t, "example.com. 60 IN A 1.1.1.1")},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "selected"},
				},
			},
		},
	}
	router := newTestRouter(t, rules, transportManager, client)

	response, err := router.Exchange(context.Background(), &mDNS.Msg{
		Question: []mDNS.Question{fixedQuestion("example.com", mDNS.TypeA)},
	}, adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("8.8.8.8")}, MessageToAddresses(response))
}

func TestExchangeLegacyDNSModeDisabledEvaluateMatchResponseRouteWithHTTPSHints(t *testing.T) {
	t.Parallel()

	transportManager := &fakeDNSTransportManager{
		defaultTransport: &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		transports: map[string]adapter.DNSTransport{
			"upstream": &fakeDNSTransport{tag: "upstream", transportType: C.DNSTypeUDP},
			"selected": &fakeDNSTransport{tag: "selected", transportType: C.DNSTypeUDP},
			"default":  &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		},
	}
	client := &fakeDNSClient{
		exchange: func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
			switch transport.Tag() {
			case "upstream":
				return fixedHTTPSHintResponse(message.Question[0], netip.MustParseAddr("1.1.1.1")), nil
			case "selected":
				return fixedHTTPSHintResponse(message.Question[0], netip.MustParseAddr("8.8.8.8")), nil
			default:
				return nil, E.New("unexpected transport")
			}
		},
	}
	rules := []option.DNSRule{
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					Domain: badoption.Listable[string]{"example.com"},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeEvaluate,
					RouteOptions: option.DNSRouteActionOptions{Server: "upstream"},
				},
			},
		},
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					MatchResponse: true,
					IPCIDR:        badoption.Listable[string]{"1.1.1.0/24"},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "selected"},
				},
			},
		},
	}
	router := newTestRouter(t, rules, transportManager, client)

	response, err := router.Exchange(context.Background(), &mDNS.Msg{
		Question: []mDNS.Question{fixedQuestion("example.com", mDNS.TypeHTTPS)},
	}, adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("8.8.8.8")}, MessageToAddresses(response))
}

func TestExchangeLegacyDNSModeDisabledEvaluateMatchResponseRouteWithMappedHTTPSIPv4Hints(t *testing.T) {
	t.Parallel()

	transportManager := &fakeDNSTransportManager{
		defaultTransport: &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		transports: map[string]adapter.DNSTransport{
			"upstream": &fakeDNSTransport{tag: "upstream", transportType: C.DNSTypeUDP},
			"selected": &fakeDNSTransport{tag: "selected", transportType: C.DNSTypeUDP},
			"default":  &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		},
	}
	client := &fakeDNSClient{
		exchange: func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
			switch transport.Tag() {
			case "upstream":
				return fixedHTTPSHintResponseWithRawHints(message.Question[0], []net.IP{net.ParseIP("1.1.1.1")}, nil), nil
			case "selected":
				return fixedHTTPSHintResponse(message.Question[0], netip.MustParseAddr("8.8.8.8")), nil
			default:
				return nil, E.New("unexpected transport")
			}
		},
	}
	rules := []option.DNSRule{
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					Domain: badoption.Listable[string]{"example.com"},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeEvaluate,
					RouteOptions: option.DNSRouteActionOptions{Server: "upstream"},
				},
			},
		},
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					MatchResponse: true,
					IPCIDR:        badoption.Listable[string]{"1.1.1.0/24"},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "selected"},
				},
			},
		},
	}
	router := newTestRouter(t, rules, transportManager, client)

	response, err := router.Exchange(context.Background(), &mDNS.Msg{
		Question: []mDNS.Question{fixedQuestion("example.com", mDNS.TypeHTTPS)},
	}, adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("8.8.8.8")}, MessageToAddresses(response))
}

func TestExchangeLegacyDNSModeDisabledEvaluateDoesNotLeakAddressesToNextQuery(t *testing.T) {
	t.Parallel()

	transportManager := &fakeDNSTransportManager{
		defaultTransport: &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		transports: map[string]adapter.DNSTransport{
			"upstream": &fakeDNSTransport{tag: "upstream", transportType: C.DNSTypeUDP},
			"selected": &fakeDNSTransport{tag: "selected", transportType: C.DNSTypeUDP},
			"default":  &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		},
	}
	var inspectedSelected bool
	client := &fakeDNSClient{
		beforeExchange: func(ctx context.Context, transport adapter.DNSTransport, message *mDNS.Msg) {
			if transport.Tag() != "selected" {
				return
			}
			inspectedSelected = true
			metadata := adapter.ContextFrom(ctx)
			require.NotNil(t, metadata)
			require.Empty(t, metadata.DestinationAddresses)
			require.NotNil(t, metadata.DNSResponse)
		},
		exchange: func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
			switch transport.Tag() {
			case "upstream":
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("1.1.1.1")}, 60), nil
			case "selected":
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("8.8.8.8")}, 60), nil
			default:
				return nil, E.New("unexpected transport")
			}
		},
	}
	rules := []option.DNSRule{
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					Domain: badoption.Listable[string]{"example.com"},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeEvaluate,
					RouteOptions: option.DNSRouteActionOptions{Server: "upstream"},
				},
			},
		},
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					MatchResponse:  true,
					ResponseAnswer: badoption.Listable[option.DNSRecordOptions]{mustRecord(t, "example.com. IN A 1.1.1.1")},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "selected"},
				},
			},
		},
	}
	router := newTestRouter(t, rules, transportManager, client)

	response, err := router.Exchange(context.Background(), &mDNS.Msg{
		Question: []mDNS.Question{fixedQuestion("example.com", mDNS.TypeA)},
	}, adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.True(t, inspectedSelected)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("8.8.8.8")}, MessageToAddresses(response))
}

func TestExchangeLegacyDNSModeDisabledEvaluateRouteResolutionFailureClearsResponse(t *testing.T) {
	t.Parallel()

	transportManager := &fakeDNSTransportManager{
		defaultTransport: &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		transports: map[string]adapter.DNSTransport{
			"upstream": &fakeDNSTransport{tag: "upstream", transportType: C.DNSTypeUDP},
			"selected": &fakeDNSTransport{tag: "selected", transportType: C.DNSTypeUDP},
			"default":  &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		},
	}
	client := &fakeDNSClient{
		exchange: func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
			switch transport.Tag() {
			case "upstream":
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("1.1.1.1")}, 60), nil
			case "selected":
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("8.8.8.8")}, 60), nil
			case "default":
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("4.4.4.4")}, 60), nil
			default:
				return nil, E.New("unexpected transport")
			}
		},
	}
	rules := []option.DNSRule{
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					Domain: badoption.Listable[string]{"example.com"},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeEvaluate,
					RouteOptions: option.DNSRouteActionOptions{Server: "upstream"},
				},
			},
		},
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					Domain: badoption.Listable[string]{"example.com"},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeEvaluate,
					RouteOptions: option.DNSRouteActionOptions{Server: "missing"},
				},
			},
		},
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					MatchResponse:  true,
					ResponseAnswer: badoption.Listable[option.DNSRecordOptions]{mustRecord(t, "example.com. IN A 1.1.1.1")},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "selected"},
				},
			},
		},
	}
	router := newTestRouter(t, rules, transportManager, client)

	response, err := router.Exchange(context.Background(), &mDNS.Msg{
		Question: []mDNS.Question{fixedQuestion("example.com", mDNS.TypeA)},
	}, adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("4.4.4.4")}, MessageToAddresses(response))
}

func TestExchangeLegacyDNSModeDisabledEvaluateExchangeFailureUsesMatchResponseBooleanSemantics(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		invert       bool
		expectedAddr netip.Addr
	}{
		{
			name:         "plain match_response rule stays false",
			expectedAddr: netip.MustParseAddr("4.4.4.4"),
		},
		{
			name:         "invert match_response rule becomes true",
			invert:       true,
			expectedAddr: netip.MustParseAddr("8.8.8.8"),
		},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			transportManager := &fakeDNSTransportManager{
				defaultTransport: &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
				transports: map[string]adapter.DNSTransport{
					"upstream": &fakeDNSTransport{tag: "upstream", transportType: C.DNSTypeUDP},
					"selected": &fakeDNSTransport{tag: "selected", transportType: C.DNSTypeUDP},
					"default":  &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
				},
			}
			client := &fakeDNSClient{
				exchange: func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
					switch transport.Tag() {
					case "upstream":
						return nil, E.New("upstream exchange failed")
					case "selected":
						return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("8.8.8.8")}, 60), nil
					case "default":
						return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("4.4.4.4")}, 60), nil
					default:
						return nil, E.New("unexpected transport")
					}
				},
			}
			rules := []option.DNSRule{
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultDNSRule{
						RawDefaultDNSRule: option.RawDefaultDNSRule{
							Domain: badoption.Listable[string]{"example.com"},
						},
						DNSRuleAction: option.DNSRuleAction{
							Action:       C.RuleActionTypeEvaluate,
							RouteOptions: option.DNSRouteActionOptions{Server: "upstream"},
						},
					},
				},
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultDNSRule{
						RawDefaultDNSRule: option.RawDefaultDNSRule{
							MatchResponse: true,
							Invert:        testCase.invert,
						},
						DNSRuleAction: option.DNSRuleAction{
							Action:       C.RuleActionTypeRoute,
							RouteOptions: option.DNSRouteActionOptions{Server: "selected"},
						},
					},
				},
			}
			router := newTestRouter(t, rules, transportManager, client)

			response, err := router.Exchange(context.Background(), &mDNS.Msg{
				Question: []mDNS.Question{fixedQuestion("example.com", mDNS.TypeA)},
			}, adapter.DNSQueryOptions{})
			require.NoError(t, err)
			require.Equal(t, []netip.Addr{testCase.expectedAddr}, MessageToAddresses(response))
		})
	}
}

func TestLookupLegacyDNSModeDisabledAllowsPartialSuccess(t *testing.T) {
	t.Parallel()

	defaultTransport := &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP}
	router := newTestRouter(t, nil, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default": defaultTransport,
		},
	}, &fakeDNSClient{
		exchange: func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
			require.Equal(t, "default", transport.Tag())
			switch message.Question[0].Qtype {
			case mDNS.TypeA:
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("1.1.1.1")}, 60), nil
			case mDNS.TypeAAAA:
				return nil, E.New("ipv6 failed")
			default:
				return nil, E.New("unexpected qtype")
			}
		},
	})
	router.currentRules.Load().legacyDNSMode = false

	addresses, err := router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("1.1.1.1")}, addresses)
}

func TestLookupLegacyDNSModeDisabledSkipsFakeIPRule(t *testing.T) {
	t.Parallel()

	defaultTransport := &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP}
	router := newTestRouter(t, []option.DNSRule{{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				Domain: badoption.Listable[string]{"example.com"},
			},
			DNSRuleAction: option.DNSRuleAction{
				Action:       C.RuleActionTypeRoute,
				RouteOptions: option.DNSRouteActionOptions{Server: "fake"},
			},
		},
	}}, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default": defaultTransport,
			"fake":    &fakeDNSTransport{tag: "fake", transportType: C.DNSTypeFakeIP},
		},
	}, &fakeDNSClient{
		exchange: func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
			require.Equal(t, "default", transport.Tag())
			if message.Question[0].Qtype == mDNS.TypeA {
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("2.2.2.2")}, 60), nil
			}
			return FixedResponse(0, message.Question[0], nil, 60), nil
		},
	})
	router.currentRules.Load().legacyDNSMode = false

	addresses, err := router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("2.2.2.2")}, addresses)
}

func TestLookupLegacyDNSModeDisabledEvaluateSkipFakeIPPreservesResponse(t *testing.T) {
	t.Parallel()

	defaultTransport := &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP}
	router := newTestRouter(t, []option.DNSRule{
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					Domain: badoption.Listable[string]{"example.com"},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeEvaluate,
					RouteOptions: option.DNSRouteActionOptions{Server: "upstream"},
				},
			},
		},
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					Domain: badoption.Listable[string]{"example.com"},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeEvaluate,
					RouteOptions: option.DNSRouteActionOptions{Server: "fake"},
				},
			},
		},
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					MatchResponse:  true,
					ResponseAnswer: badoption.Listable[option.DNSRecordOptions]{mustRecord(t, "example.com. IN A 1.1.1.1")},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "selected"},
				},
			},
		},
	}, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default":  defaultTransport,
			"upstream": &fakeDNSTransport{tag: "upstream", transportType: C.DNSTypeUDP},
			"fake":     &fakeDNSTransport{tag: "fake", transportType: C.DNSTypeFakeIP},
			"selected": &fakeDNSTransport{tag: "selected", transportType: C.DNSTypeUDP},
		},
	}, &fakeDNSClient{
		exchange: func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
			switch transport.Tag() {
			case "upstream":
				if message.Question[0].Qtype == mDNS.TypeA {
					return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("1.1.1.1")}, 60), nil
				}
				return FixedResponse(0, message.Question[0], nil, 60), nil
			case "selected":
				if message.Question[0].Qtype == mDNS.TypeA {
					return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("2.2.2.2")}, 60), nil
				}
				return FixedResponse(0, message.Question[0], nil, 60), nil
			case "default":
				if message.Question[0].Qtype == mDNS.TypeA {
					return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("4.4.4.4")}, 60), nil
				}
				return FixedResponse(0, message.Question[0], nil, 60), nil
			default:
				return nil, E.New("unexpected transport")
			}
		},
	})
	router.currentRules.Load().legacyDNSMode = false

	addresses, err := router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("2.2.2.2")}, addresses)
}

func TestLookupLegacyDNSModeDisabledUsesQueryTypeRule(t *testing.T) {
	t.Parallel()

	defaultTransport := &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP}
	router := newTestRouter(t, []option.DNSRule{{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				QueryType: badoption.Listable[option.DNSQueryType]{option.DNSQueryType(mDNS.TypeA)},
			},
			DNSRuleAction: option.DNSRuleAction{
				Action:       C.RuleActionTypeRoute,
				RouteOptions: option.DNSRouteActionOptions{Server: "only-a"},
			},
		},
	}}, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default": defaultTransport,
			"only-a":  &fakeDNSTransport{tag: "only-a", transportType: C.DNSTypeUDP},
		},
	}, &fakeDNSClient{
		exchange: func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
			switch transport.Tag() {
			case "default":
				if message.Question[0].Qtype == mDNS.TypeA {
					return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("3.3.3.3")}, 60), nil
				}
				return FixedResponse(0, message.Question[0], nil, 60), nil
			case "only-a":
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("9.9.9.9")}, 60), nil
			default:
				return nil, E.New("unexpected transport")
			}
		},
	})
	require.False(t, router.currentRules.Load().legacyDNSMode)

	addresses, err := router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("9.9.9.9")}, addresses)
}

func TestLookupLegacyDNSModeDisabledUsesRuleSetQueryTypeRule(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ruleSet, err := rulepkg.NewRuleSet(ctx, log.NewNOPFactory().NewLogger("router"), option.RuleSet{
		Type: C.RuleSetTypeInline,
		Tag:  "query-set",
		InlineOptions: option.PlainRuleSet{
			Rules: []option.HeadlessRule{{
				Type: C.RuleTypeDefault,
				DefaultOptions: option.DefaultHeadlessRule{
					QueryType: badoption.Listable[option.DNSQueryType]{option.DNSQueryType(mDNS.TypeA)},
				},
			}},
		},
	})
	require.NoError(t, err)
	ctx = service.ContextWith[adapter.Router](ctx, &fakeRouter{
		ruleSets: map[string]adapter.RuleSet{
			"query-set": ruleSet,
		},
	})

	defaultTransport := &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP}
	router := newTestRouterWithContext(t, ctx, []option.DNSRule{{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				RuleSet: badoption.Listable[string]{"query-set"},
			},
			DNSRuleAction: option.DNSRuleAction{
				Action:       C.RuleActionTypeRoute,
				RouteOptions: option.DNSRouteActionOptions{Server: "only-a"},
			},
		},
	}}, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default": defaultTransport,
			"only-a":  &fakeDNSTransport{tag: "only-a", transportType: C.DNSTypeUDP},
		},
	}, &fakeDNSClient{
		exchange: func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
			switch transport.Tag() {
			case "default":
				if message.Question[0].Qtype == mDNS.TypeA {
					return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("3.3.3.3")}, 60), nil
				}
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("2001:db8::4")}, 60), nil
			case "only-a":
				if message.Question[0].Qtype == mDNS.TypeA {
					return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("9.9.9.9")}, 60), nil
				}
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("2001:db8::9")}, 60), nil
			default:
				return nil, E.New("unexpected transport")
			}
		},
	})
	require.False(t, router.currentRules.Load().legacyDNSMode)

	addresses, err := router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{
		netip.MustParseAddr("9.9.9.9"),
		netip.MustParseAddr("2001:db8::4"),
	}, addresses)
}

func TestLookupLegacyDNSModeDisabledUsesIPVersionRule(t *testing.T) {
	t.Parallel()

	defaultTransport := &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP}
	router := newTestRouter(t, []option.DNSRule{{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				IPVersion: 6,
			},
			DNSRuleAction: option.DNSRuleAction{
				Action:       C.RuleActionTypeRoute,
				RouteOptions: option.DNSRouteActionOptions{Server: "only-v6"},
			},
		},
	}}, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default": defaultTransport,
			"only-v6": &fakeDNSTransport{tag: "only-v6", transportType: C.DNSTypeUDP},
		},
	}, &fakeDNSClient{
		exchange: func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
			switch transport.Tag() {
			case "default":
				if message.Question[0].Qtype == mDNS.TypeA {
					return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("3.3.3.3")}, 60), nil
				}
				return FixedResponse(0, message.Question[0], nil, 60), nil
			case "only-v6":
				if message.Question[0].Qtype == mDNS.TypeAAAA {
					return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("2001:db8::9")}, 60), nil
				}
				return FixedResponse(0, message.Question[0], nil, 60), nil
			default:
				return nil, E.New("unexpected transport")
			}
		},
	})
	require.False(t, router.currentRules.Load().legacyDNSMode)

	addresses, err := router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("3.3.3.3"), netip.MustParseAddr("2001:db8::9")}, addresses)
}

func TestInitializeRejectsDNSRuleStrategyWhenLegacyDNSModeIsDisabledByEvaluate(t *testing.T) {
	t.Parallel()

	router := &Router{
		ctx:                   context.Background(),
		logger:                log.NewNOPFactory().NewLogger("dns"),
		transport:             &fakeDNSTransportManager{},
		client:                &fakeDNSClient{},
		rawRules:              make([]option.DNSRule, 0, 1),
		defaultDomainStrategy: C.DomainStrategyAsIS,
	}
	router.currentRules.Store(newRulesSnapshot(make([]adapter.DNSRule, 0, 1), false))
	err := router.Initialize([]option.DNSRule{{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				Domain: badoption.Listable[string]{"example.com"},
			},
			DNSRuleAction: option.DNSRuleAction{
				Action: C.RuleActionTypeEvaluate,
				RouteOptions: option.DNSRouteActionOptions{
					Server:   "default",
					Strategy: option.DomainStrategy(C.DomainStrategyIPv4Only),
				},
			},
		},
	}})
	require.ErrorContains(t, err, "legacyDNSMode")
}

func TestInitializeRejectsDNSRuleStrategyWhenLegacyDNSModeIsDisabledByMatchResponse(t *testing.T) {
	t.Parallel()

	router := &Router{
		ctx:                   context.Background(),
		logger:                log.NewNOPFactory().NewLogger("dns"),
		transport:             &fakeDNSTransportManager{},
		client:                &fakeDNSClient{},
		rawRules:              make([]option.DNSRule, 0, 1),
		defaultDomainStrategy: C.DomainStrategyAsIS,
	}
	router.currentRules.Store(newRulesSnapshot(make([]adapter.DNSRule, 0, 1), false))
	err := router.Initialize([]option.DNSRule{{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				MatchResponse: true,
			},
			DNSRuleAction: option.DNSRuleAction{
				Action: C.RuleActionTypeRouteOptions,
				RouteOptionsOptions: option.DNSRouteOptionsActionOptions{
					Strategy: option.DomainStrategy(C.DomainStrategyIPv4Only),
				},
			},
		},
	}})
	require.ErrorContains(t, err, "legacyDNSMode")
}

func TestLookupLegacyDNSModeUsesRouteStrategy(t *testing.T) {
	t.Parallel()

	defaultTransport := &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP}
	selectedTransport := &fakeDNSTransport{tag: "selected", transportType: C.DNSTypeUDP}
	router := newTestRouter(t, []option.DNSRule{{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				Domain: badoption.Listable[string]{"example.com"},
			},
			DNSRuleAction: option.DNSRuleAction{
				Action: C.RuleActionTypeRoute,
				RouteOptions: option.DNSRouteActionOptions{
					Server:   "selected",
					Strategy: option.DomainStrategy(C.DomainStrategyIPv4Only),
				},
			},
		},
	}}, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default":  defaultTransport,
			"selected": selectedTransport,
		},
	}, &fakeDNSClient{
		lookup: func(transport adapter.DNSTransport, domain string, options adapter.DNSQueryOptions) ([]netip.Addr, *mDNS.Msg, error) {
			require.Equal(t, "selected", transport.Tag())
			require.Equal(t, C.DomainStrategyIPv4Only, options.Strategy)
			return []netip.Addr{netip.MustParseAddr("2.2.2.2")}, nil, nil
		},
	})

	require.True(t, router.currentRules.Load().legacyDNSMode)

	addresses, err := router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("2.2.2.2")}, addresses)
}

func TestLookupLegacyDNSModeDisabledReturnsRejectedErrorForRejectAction(t *testing.T) {
	t.Parallel()

	defaultTransport := &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP}
	router := newTestRouter(t, []option.DNSRule{
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					Domain: badoption.Listable[string]{"example.com"},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action: C.RuleActionTypeReject,
					RejectOptions: option.RejectActionOptions{
						Method: C.RuleActionRejectMethodDefault,
					},
				},
			},
		},
	}, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default": defaultTransport,
		},
	}, &fakeDNSClient{})
	require.False(t, router.currentRules.Load().legacyDNSMode)

	addresses, err := router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{})
	require.Nil(t, addresses)
	require.Error(t, err)
	require.True(t, rulepkg.IsRejected(err))
}

func TestExchangeLegacyDNSModeDisabledReturnsRefusedResponseForRejectAction(t *testing.T) {
	t.Parallel()

	defaultTransport := &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP}
	router := newTestRouter(t, []option.DNSRule{
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					Domain: badoption.Listable[string]{"example.com"},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action: C.RuleActionTypeReject,
					RejectOptions: option.RejectActionOptions{
						Method: C.RuleActionRejectMethodDefault,
					},
				},
			},
		},
	}, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default": defaultTransport,
		},
	}, &fakeDNSClient{})
	require.False(t, router.currentRules.Load().legacyDNSMode)

	response, err := router.Exchange(context.Background(), &mDNS.Msg{
		Question: []mDNS.Question{fixedQuestion("example.com", mDNS.TypeA)},
	}, adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, mDNS.RcodeRefused, response.Rcode)
	require.Equal(t, []mDNS.Question{fixedQuestion("example.com", mDNS.TypeA)}, response.Question)
}

func TestLookupLegacyDNSModeDisabledFiltersPerQueryTypeAddressesBeforeMerging(t *testing.T) {
	t.Parallel()

	defaultTransport := &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP}
	router := newTestRouter(t, []option.DNSRule{
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					Domain: badoption.Listable[string]{"example.com"},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action: C.RuleActionTypePredefined,
					PredefinedOptions: option.DNSRouteActionPredefined{
						Answer: badoption.Listable[option.DNSRecordOptions]{
							mustRecord(t, "example.com. IN A 1.1.1.1"),
							mustRecord(t, "example.com. IN AAAA 2001:db8::1"),
						},
					},
				},
			},
		},
	}, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default": defaultTransport,
		},
	}, &fakeDNSClient{})
	require.False(t, router.currentRules.Load().legacyDNSMode)

	addresses, err := router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{
		netip.MustParseAddr("1.1.1.1"),
		netip.MustParseAddr("2001:db8::1"),
	}, addresses)
}

func TestLookupLegacyDNSModeDisabledUsesInputStrategy(t *testing.T) {
	t.Parallel()

	defaultTransport := &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP}
	var queryTypeAccess sync.Mutex
	var queryTypes []uint16
	recordQueryType := func(queryType uint16) {
		queryTypeAccess.Lock()
		queryTypes = append(queryTypes, queryType)
		queryTypeAccess.Unlock()
	}
	currentQueryTypes := func() []uint16 {
		queryTypeAccess.Lock()
		defer queryTypeAccess.Unlock()
		return append([]uint16(nil), queryTypes...)
	}
	router := newTestRouter(t, nil, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default": defaultTransport,
		},
	}, &fakeDNSClient{
		exchange: func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
			recordQueryType(message.Question[0].Qtype)
			if message.Question[0].Qtype == mDNS.TypeA {
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("2.2.2.2")}, 60), nil
			}
			return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("2001:db8::2")}, 60), nil
		},
	})
	router.currentRules.Load().legacyDNSMode = false

	addresses, err := router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{
		Strategy: C.DomainStrategyIPv4Only,
	})
	require.NoError(t, err)
	require.Equal(t, []uint16{mDNS.TypeA}, currentQueryTypes())
	require.Equal(t, []netip.Addr{netip.MustParseAddr("2.2.2.2")}, addresses)
}

func TestLookupLegacyDNSModeDisabledUsesDefaultStrategy(t *testing.T) {
	t.Parallel()

	defaultTransport := &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP}
	var queryTypeAccess sync.Mutex
	var queryTypes []uint16
	recordQueryType := func(queryType uint16) {
		queryTypeAccess.Lock()
		queryTypes = append(queryTypes, queryType)
		queryTypeAccess.Unlock()
	}
	currentQueryTypes := func() []uint16 {
		queryTypeAccess.Lock()
		defer queryTypeAccess.Unlock()
		return append([]uint16(nil), queryTypes...)
	}
	router := newTestRouter(t, nil, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default": defaultTransport,
		},
	}, &fakeDNSClient{
		exchange: func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
			recordQueryType(message.Question[0].Qtype)
			if message.Question[0].Qtype == mDNS.TypeA {
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("2.2.2.2")}, 60), nil
			}
			return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("2001:db8::2")}, 60), nil
		},
	})
	router.defaultDomainStrategy = C.DomainStrategyIPv4Only
	router.currentRules.Load().legacyDNSMode = false

	addresses, err := router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, []uint16{mDNS.TypeA}, currentQueryTypes())
	require.Equal(t, []netip.Addr{netip.MustParseAddr("2.2.2.2")}, addresses)
}

func TestExchangeLegacyDNSModeDisabledLogicalMatchResponseIPCIDRFallsThrough(t *testing.T) {
	t.Parallel()

	transportManager := &fakeDNSTransportManager{
		defaultTransport: &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		transports: map[string]adapter.DNSTransport{
			"upstream": &fakeDNSTransport{tag: "upstream", transportType: C.DNSTypeUDP},
			"selected": &fakeDNSTransport{tag: "selected", transportType: C.DNSTypeUDP},
			"default":  &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		},
	}
	client := &fakeDNSClient{
		exchange: func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
			switch transport.Tag() {
			case "upstream":
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("9.9.9.9")}, 60), nil
			case "selected":
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("8.8.8.8")}, 60), nil
			case "default":
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("4.4.4.4")}, 60), nil
			default:
				return nil, E.New("unexpected transport")
			}
		},
	}
	rules := []option.DNSRule{
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					Domain: badoption.Listable[string]{"example.com"},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeEvaluate,
					RouteOptions: option.DNSRouteActionOptions{Server: "upstream"},
				},
			},
		},
		{
			Type: C.RuleTypeLogical,
			LogicalOptions: option.LogicalDNSRule{
				RawLogicalDNSRule: option.RawLogicalDNSRule{
					Mode: C.LogicalTypeOr,
					Rules: []option.DNSRule{{
						Type: C.RuleTypeDefault,
						DefaultOptions: option.DefaultDNSRule{
							RawDefaultDNSRule: option.RawDefaultDNSRule{
								MatchResponse: true,
								IPCIDR:        badoption.Listable[string]{"1.1.1.0/24"},
							},
						},
					}},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "selected"},
				},
			},
		},
	}
	router := newTestRouter(t, rules, transportManager, client)

	response, err := router.Exchange(context.Background(), &mDNS.Msg{
		Question: []mDNS.Question{fixedQuestion("example.com", mDNS.TypeA)},
	}, adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("4.4.4.4")}, MessageToAddresses(response))
}

func TestLegacyDNSModeReportsLegacyAddressFilterDeprecation(t *testing.T) {
	t.Parallel()

	manager := &fakeDeprecatedManager{}
	ctx := service.ContextWith[deprecated.Manager](context.Background(), manager)
	router := &Router{
		ctx:                   ctx,
		logger:                log.NewNOPFactory().NewLogger("dns"),
		client:                &fakeDNSClient{},
		defaultDomainStrategy: C.DomainStrategyAsIS,
	}
	router.currentRules.Store(newRulesSnapshot(make([]adapter.DNSRule, 0, 1), false))
	err := router.Initialize([]option.DNSRule{{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				IPCIDR: badoption.Listable[string]{"1.1.1.0/24"},
			},
			DNSRuleAction: option.DNSRuleAction{
				Action:       C.RuleActionTypeRoute,
				RouteOptions: option.DNSRouteActionOptions{Server: "default"},
			},
		},
	}})
	require.NoError(t, err)

	err = router.Start(adapter.StartStateStart)
	require.NoError(t, err)
	require.Len(t, manager.features, 1)
	require.Equal(t, deprecated.OptionLegacyDNSAddressFilter.Name, manager.features[0].Name)
}

func TestLegacyDNSModeReportsDNSRuleStrategyDeprecation(t *testing.T) {
	t.Parallel()

	manager := &fakeDeprecatedManager{}
	ctx := service.ContextWith[deprecated.Manager](context.Background(), manager)
	router := &Router{
		ctx:                   ctx,
		logger:                log.NewNOPFactory().NewLogger("dns"),
		client:                &fakeDNSClient{},
		defaultDomainStrategy: C.DomainStrategyAsIS,
	}
	router.currentRules.Store(newRulesSnapshot(make([]adapter.DNSRule, 0, 1), false))
	err := router.Initialize([]option.DNSRule{{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				Domain: badoption.Listable[string]{"example.com"},
			},
			DNSRuleAction: option.DNSRuleAction{
				Action: C.RuleActionTypeRoute,
				RouteOptions: option.DNSRouteActionOptions{
					Server:   "default",
					Strategy: option.DomainStrategy(C.DomainStrategyIPv4Only),
				},
			},
		},
	}})
	require.NoError(t, err)

	err = router.Start(adapter.StartStateStart)
	require.NoError(t, err)
	require.Len(t, manager.features, 1)
	require.Equal(t, deprecated.OptionLegacyDNSRuleStrategy.Name, manager.features[0].Name)
}
