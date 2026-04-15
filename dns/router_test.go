package dns

import (
	"context"
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
	lookupWithCtx  func(ctx context.Context, transport adapter.DNSTransport, domain string, options adapter.DNSQueryOptions) ([]netip.Addr, *mDNS.Msg, error)
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
	if c.exchange == nil {
		if len(message.Question) != 1 {
			return nil, E.New("unused client exchange")
		}
		var (
			addresses []netip.Addr
			response  *mDNS.Msg
			err       error
		)
		if c.lookupWithCtx != nil {
			addresses, response, err = c.lookupWithCtx(ctx, transport, FqdnToDomain(message.Question[0].Name), adapter.DNSQueryOptions{})
		} else if c.lookup != nil {
			addresses, response, err = c.lookup(transport, FqdnToDomain(message.Question[0].Name), adapter.DNSQueryOptions{})
		} else {
			return nil, E.New("unused client exchange")
		}
		if err != nil {
			return nil, err
		}
		if response != nil {
			return response, nil
		}
		return FixedResponse(0, message.Question[0], addresses, 60), nil
	}
	return c.exchange(transport, message)
}

func (c *fakeDNSClient) Lookup(ctx context.Context, transport adapter.DNSTransport, domain string, options adapter.DNSQueryOptions, responseChecker func(*mDNS.Msg) bool) ([]netip.Addr, error) {
	if c.lookup == nil && c.lookupWithCtx == nil {
		return nil, E.New("unused client lookup")
	}
	var (
		addresses []netip.Addr
		response  *mDNS.Msg
		err       error
	)
	if c.lookupWithCtx != nil {
		addresses, response, err = c.lookupWithCtx(ctx, transport, domain, options)
	} else {
		addresses, response, err = c.lookup(transport, domain, options)
	}
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
	router := newTestRouterWithContext(t, context.Background(), rules, transportManager, client)
	t.Cleanup(func() {
		router.Close()
	})
	return router
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
		rules:                 make([]adapter.DNSRule, 0, len(rules)),
		defaultDomainStrategy: C.DomainStrategyAsIS,
	}
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
	require.ErrorContains(t, err, "Response Match Fields")
	require.ErrorContains(t, err, "require match_response")
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

	require.True(t, router.legacyDNSMode)

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

func TestValidateRuleSetMetadataUpdateRejectsRuleSetThatWouldDisableLegacyDNSMode(t *testing.T) {
	t.Parallel()

	fakeSet := &fakeRuleSet{
		metadata: adapter.RuleSetMetadata{
			ContainsIPCIDRRule: true,
		},
	}
	routerService := &fakeRouter{
		ruleSets: map[string]adapter.RuleSet{
			"dynamic-set": fakeSet,
		},
	}
	ctx := service.ContextWith[adapter.Router](context.Background(), routerService)
	router := newTestRouterWithContext(t, ctx, []option.DNSRule{
		{
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
		},
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					IPIsPrivate: true,
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "selected"},
				},
			},
		},
	}, &fakeDNSTransportManager{
		defaultTransport: &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		transports: map[string]adapter.DNSTransport{
			"default":  &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
			"selected": &fakeDNSTransport{tag: "selected", transportType: C.DNSTypeUDP},
		},
	}, &fakeDNSClient{
		lookup: func(adapter.DNSTransport, string, adapter.DNSQueryOptions) ([]netip.Addr, *mDNS.Msg, error) {
			return []netip.Addr{netip.MustParseAddr("10.0.0.1")}, nil, nil
		},
	})
	require.True(t, router.legacyDNSMode)

	err := router.ValidateRuleSetMetadataUpdate("dynamic-set", adapter.RuleSetMetadata{
		ContainsDNSQueryTypeRule: true,
	})
	require.ErrorContains(t, err, "require match_response")
}

func TestValidateRuleSetMetadataUpdateRejectsRuleSetOnlyLegacyModeSwitchToNew(t *testing.T) {
	t.Parallel()

	fakeSet := &fakeRuleSet{
		metadata: adapter.RuleSetMetadata{
			ContainsIPCIDRRule: true,
		},
	}
	routerService := &fakeRouter{
		ruleSets: map[string]adapter.RuleSet{
			"dynamic-set": fakeSet,
		},
	}
	ctx := service.ContextWith[adapter.Router](context.Background(), routerService)
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
		defaultTransport: &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		transports: map[string]adapter.DNSTransport{
			"default":  &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
			"selected": &fakeDNSTransport{tag: "selected", transportType: C.DNSTypeUDP},
		},
	}, &fakeDNSClient{
		lookup: func(adapter.DNSTransport, string, adapter.DNSQueryOptions) ([]netip.Addr, *mDNS.Msg, error) {
			return []netip.Addr{netip.MustParseAddr("10.0.0.1")}, nil, nil
		},
	})
	require.True(t, router.legacyDNSMode)

	err := router.ValidateRuleSetMetadataUpdate("dynamic-set", adapter.RuleSetMetadata{
		ContainsIPCIDRRule:       true,
		ContainsDNSQueryTypeRule: true,
	})
	require.ErrorContains(t, err, "Address Filter Fields")
}

func TestValidateRuleSetMetadataUpdateBeforeStartUsesStartupValidation(t *testing.T) {
	t.Parallel()

	fakeSet := &fakeRuleSet{}
	routerService := &fakeRouter{
		ruleSets: map[string]adapter.RuleSet{
			"dynamic-set": fakeSet,
		},
	}
	ctx := service.ContextWith[adapter.Router](context.Background(), routerService)
	router := &Router{
		ctx:                   ctx,
		logger:                log.NewNOPFactory().NewLogger("dns"),
		transport:             &fakeDNSTransportManager{},
		client:                &fakeDNSClient{},
		rawRules:              make([]option.DNSRule, 0, 2),
		rules:                 make([]adapter.DNSRule, 0, 2),
		defaultDomainStrategy: C.DomainStrategyAsIS,
	}
	err := router.Initialize([]option.DNSRule{
		{
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
		},
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					IPIsPrivate: true,
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "selected"},
				},
			},
		},
	})
	require.NoError(t, err)
	require.False(t, router.started)

	err = router.ValidateRuleSetMetadataUpdate("dynamic-set", adapter.RuleSetMetadata{
		ContainsDNSQueryTypeRule: true,
	})
	require.ErrorContains(t, err, "require match_response")
}

func TestValidateRuleSetMetadataUpdateRejectsRuleSetThatWouldRequireLegacyDNSMode(t *testing.T) {
	t.Parallel()

	fakeSet := &fakeRuleSet{}
	routerService := &fakeRouter{
		ruleSets: map[string]adapter.RuleSet{
			"dynamic-set": fakeSet,
		},
	}
	ctx := service.ContextWith[adapter.Router](context.Background(), routerService)
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
		defaultTransport: &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		transports: map[string]adapter.DNSTransport{
			"default":  &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
			"selected": &fakeDNSTransport{tag: "selected", transportType: C.DNSTypeUDP},
		},
	}, &fakeDNSClient{
		lookup: func(adapter.DNSTransport, string, adapter.DNSQueryOptions) ([]netip.Addr, *mDNS.Msg, error) {
			return []netip.Addr{netip.MustParseAddr("1.1.1.1")}, nil, nil
		},
	})
	require.False(t, router.legacyDNSMode)

	err := router.ValidateRuleSetMetadataUpdate("dynamic-set", adapter.RuleSetMetadata{
		ContainsIPCIDRRule: true,
	})
	require.ErrorContains(t, err, "Address Filter Fields")
}

func TestValidateRuleSetMetadataUpdateAllowsRuleSetThatKeepsNonLegacyDNSMode(t *testing.T) {
	t.Parallel()

	fakeSet := &fakeRuleSet{}
	routerService := &fakeRouter{
		ruleSets: map[string]adapter.RuleSet{
			"dynamic-set": fakeSet,
		},
	}
	ctx := service.ContextWith[adapter.Router](context.Background(), routerService)
	router := newTestRouterWithContext(t, ctx, []option.DNSRule{
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
					RuleSet: badoption.Listable[string]{"dynamic-set"},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "selected"},
				},
			},
		},
	}, &fakeDNSTransportManager{
		defaultTransport: &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		transports: map[string]adapter.DNSTransport{
			"default":  &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
			"selected": &fakeDNSTransport{tag: "selected", transportType: C.DNSTypeUDP},
			"upstream": &fakeDNSTransport{tag: "upstream", transportType: C.DNSTypeUDP},
		},
	}, &fakeDNSClient{})
	require.False(t, router.legacyDNSMode)

	err := router.ValidateRuleSetMetadataUpdate("dynamic-set", adapter.RuleSetMetadata{
		ContainsIPCIDRRule:    true,
		ContainsNonIPCIDRRule: true,
	})
	require.NoError(t, err)
}

func TestValidateRuleSetMetadataUpdateAllowsRelaxingLegacyRequirement(t *testing.T) {
	t.Parallel()

	fakeSet := &fakeRuleSet{
		metadata: adapter.RuleSetMetadata{
			ContainsIPCIDRRule: true,
		},
	}
	routerService := &fakeRouter{
		ruleSets: map[string]adapter.RuleSet{
			"dynamic-set": fakeSet,
		},
	}
	ctx := service.ContextWith[adapter.Router](context.Background(), routerService)
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
		defaultTransport: &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		transports: map[string]adapter.DNSTransport{
			"default":  &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
			"selected": &fakeDNSTransport{tag: "selected", transportType: C.DNSTypeUDP},
		},
	}, &fakeDNSClient{
		lookup: func(adapter.DNSTransport, string, adapter.DNSQueryOptions) ([]netip.Addr, *mDNS.Msg, error) {
			return []netip.Addr{netip.MustParseAddr("10.0.0.1")}, nil, nil
		},
	})
	require.True(t, router.legacyDNSMode)

	err := router.ValidateRuleSetMetadataUpdate("dynamic-set", adapter.RuleSetMetadata{})
	require.NoError(t, err)
}

func TestInitializeRejectsPureIPRuleSetWhenLegacyDNSModeDisabled(t *testing.T) {
	t.Parallel()

	fakeSet := &fakeRuleSet{
		metadata: adapter.RuleSetMetadata{
			ContainsIPCIDRRule: true,
		},
	}
	routerService := &fakeRouter{
		ruleSets: map[string]adapter.RuleSet{
			"pure-ip": fakeSet,
		},
	}
	ctx := service.ContextWith[adapter.Router](context.Background(), routerService)
	router := &Router{
		ctx:                   ctx,
		logger:                log.NewNOPFactory().NewLogger("dns"),
		transport:             &fakeDNSTransportManager{},
		client:                &fakeDNSClient{},
		rawRules:              make([]option.DNSRule, 0, 2),
		rules:                 make([]adapter.DNSRule, 0, 2),
		defaultDomainStrategy: C.DomainStrategyAsIS,
	}
	err := router.Initialize([]option.DNSRule{
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					QueryType: badoption.Listable[option.DNSQueryType]{option.DNSQueryType(mDNS.TypeA)},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "selected"},
				},
			},
		},
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					RuleSet: badoption.Listable[string]{"pure-ip"},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "selected"},
				},
			},
		},
	})
	require.ErrorContains(t, err, "Address Filter Fields")
}

func TestInitializeAllowsMixedRuleSetWhenLegacyDNSModeDisabled(t *testing.T) {
	t.Parallel()

	fakeSet := &fakeRuleSet{
		metadata: adapter.RuleSetMetadata{
			ContainsIPCIDRRule:    true,
			ContainsNonIPCIDRRule: true,
		},
	}
	routerService := &fakeRouter{
		ruleSets: map[string]adapter.RuleSet{
			"mixed": fakeSet,
		},
	}
	ctx := service.ContextWith[adapter.Router](context.Background(), routerService)
	router := newTestRouterWithContext(t, ctx, []option.DNSRule{
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					QueryType: badoption.Listable[option.DNSQueryType]{option.DNSQueryType(mDNS.TypeA)},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "selected"},
				},
			},
		},
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					RuleSet: badoption.Listable[string]{"mixed"},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "selected"},
				},
			},
		},
	}, &fakeDNSTransportManager{
		defaultTransport: &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		transports: map[string]adapter.DNSTransport{
			"default":  &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
			"selected": &fakeDNSTransport{tag: "selected", transportType: C.DNSTypeUDP},
		},
	}, &fakeDNSClient{})
	require.False(t, router.legacyDNSMode)
}

func TestValidateRuleSetMetadataUpdateRejectsRuleSetFlippingToPureIP(t *testing.T) {
	t.Parallel()

	fakeSet := &fakeRuleSet{
		metadata: adapter.RuleSetMetadata{
			ContainsIPCIDRRule:    true,
			ContainsNonIPCIDRRule: true,
		},
	}
	routerService := &fakeRouter{
		ruleSets: map[string]adapter.RuleSet{
			"mixed": fakeSet,
		},
	}
	ctx := service.ContextWith[adapter.Router](context.Background(), routerService)
	router := newTestRouterWithContext(t, ctx, []option.DNSRule{
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					QueryType: badoption.Listable[option.DNSQueryType]{option.DNSQueryType(mDNS.TypeA)},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "selected"},
				},
			},
		},
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					RuleSet: badoption.Listable[string]{"mixed"},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "selected"},
				},
			},
		},
	}, &fakeDNSTransportManager{
		defaultTransport: &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		transports: map[string]adapter.DNSTransport{
			"default":  &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
			"selected": &fakeDNSTransport{tag: "selected", transportType: C.DNSTypeUDP},
		},
	}, &fakeDNSClient{})
	require.False(t, router.legacyDNSMode)

	err := router.ValidateRuleSetMetadataUpdate("mixed", adapter.RuleSetMetadata{
		ContainsIPCIDRRule: true,
	})
	require.ErrorContains(t, err, "Address Filter Fields")
}

func TestCloseWaitsForInFlightLookupUntilContextCancellation(t *testing.T) {
	t.Parallel()

	defaultTransport := &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP}
	selectedTransport := &fakeDNSTransport{tag: "selected", transportType: C.DNSTypeUDP}
	lookupStarted := make(chan struct{})
	var lookupStartedOnce sync.Once
	router := newTestRouter(t, []option.DNSRule{{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				Domain: badoption.Listable[string]{"example.com"},
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
		lookupWithCtx: func(ctx context.Context, transport adapter.DNSTransport, domain string, options adapter.DNSQueryOptions) ([]netip.Addr, *mDNS.Msg, error) {
			require.Equal(t, "selected", transport.Tag())
			require.Equal(t, "example.com", domain)
			lookupStartedOnce.Do(func() {
				close(lookupStarted)
			})
			<-ctx.Done()
			return nil, nil, ctx.Err()
		},
	})

	lookupCtx, cancelLookup := context.WithCancel(context.Background())
	defer cancelLookup()
	var (
		lookupErr error
		closeErr  error
	)
	lookupDone := make(chan struct{})
	go func() {
		_, lookupErr = router.Lookup(lookupCtx, "example.com", adapter.DNSQueryOptions{})
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

	select {
	case <-closeDone:
		t.Fatal("close finished before lookup context cancellation")
	default:
	}

	cancelLookup()

	select {
	case <-lookupDone:
	case <-time.After(time.Second):
		t.Fatal("lookup did not finish after cancellation")
	}
	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatal("close did not finish after lookup cancellation")
	}

	require.ErrorIs(t, lookupErr, context.Canceled)
	require.NoError(t, closeErr)
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

	require.True(t, router.legacyDNSMode)

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

	require.True(t, router.legacyDNSMode)

	addresses, err := router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{
		LookupStrategy: C.DomainStrategyIPv4Only,
	})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("9.9.9.9")}, addresses)
	require.Equal(t, []string{"private", "default"}, currentLookupTags())
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

func TestExchangeLegacyDNSModeDisabledEvaluateMatchResponseRcodeRoute(t *testing.T) {
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
				return &mDNS.Msg{
					MsgHdr: mDNS.MsgHdr{
						Response: true,
						Rcode:    mDNS.RcodeNameError,
					},
					Question: []mDNS.Question{message.Question[0]},
				}, nil
			case "selected":
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("8.8.8.8")}, 60), nil
			default:
				return nil, E.New("unexpected transport")
			}
		},
	}
	rcode := option.DNSRCode(mDNS.RcodeNameError)
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
					ResponseRcode: &rcode,
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

func TestExchangeLegacyDNSModeDisabledEvaluateMatchResponseNsRoute(t *testing.T) {
	t.Parallel()

	transportManager := &fakeDNSTransportManager{
		defaultTransport: &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		transports: map[string]adapter.DNSTransport{
			"upstream": &fakeDNSTransport{tag: "upstream", transportType: C.DNSTypeUDP},
			"selected": &fakeDNSTransport{tag: "selected", transportType: C.DNSTypeUDP},
			"default":  &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		},
	}
	nsRecord := mustRecord(t, "example.com. IN NS ns1.example.com.")
	client := &fakeDNSClient{
		exchange: func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
			switch transport.Tag() {
			case "upstream":
				return &mDNS.Msg{
					MsgHdr: mDNS.MsgHdr{
						Response: true,
						Rcode:    mDNS.RcodeSuccess,
					},
					Question: []mDNS.Question{message.Question[0]},
					Ns:       []mDNS.RR{nsRecord.Build()},
				}, nil
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
					MatchResponse: true,
					ResponseNs:    badoption.Listable[option.DNSRecordOptions]{nsRecord},
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

func TestExchangeLegacyDNSModeDisabledEvaluateMatchResponseExtraRoute(t *testing.T) {
	t.Parallel()

	transportManager := &fakeDNSTransportManager{
		defaultTransport: &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		transports: map[string]adapter.DNSTransport{
			"upstream": &fakeDNSTransport{tag: "upstream", transportType: C.DNSTypeUDP},
			"selected": &fakeDNSTransport{tag: "selected", transportType: C.DNSTypeUDP},
			"default":  &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		},
	}
	extraRecord := mustRecord(t, "ns1.example.com. IN A 192.0.2.53")
	client := &fakeDNSClient{
		exchange: func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
			switch transport.Tag() {
			case "upstream":
				return &mDNS.Msg{
					MsgHdr: mDNS.MsgHdr{
						Response: true,
						Rcode:    mDNS.RcodeSuccess,
					},
					Question: []mDNS.Question{message.Question[0]},
					Extra:    []mDNS.RR{extraRecord.Build()},
				}, nil
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
					MatchResponse: true,
					ResponseExtra: badoption.Listable[option.DNSRecordOptions]{extraRecord},
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

func TestExchangeLegacyDNSModeDisabledSecondEvaluateOverwritesFirstResponse(t *testing.T) {
	t.Parallel()

	transportManager := &fakeDNSTransportManager{
		defaultTransport: &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		transports: map[string]adapter.DNSTransport{
			"default":         &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
			"first-upstream":  &fakeDNSTransport{tag: "first-upstream", transportType: C.DNSTypeUDP},
			"second-upstream": &fakeDNSTransport{tag: "second-upstream", transportType: C.DNSTypeUDP},
			"first-match":     &fakeDNSTransport{tag: "first-match", transportType: C.DNSTypeUDP},
			"second-match":    &fakeDNSTransport{tag: "second-match", transportType: C.DNSTypeUDP},
		},
	}
	client := &fakeDNSClient{
		exchange: func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
			switch transport.Tag() {
			case "first-upstream":
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("1.1.1.1")}, 60), nil
			case "second-upstream":
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("2.2.2.2")}, 60), nil
			case "first-match":
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("7.7.7.7")}, 60), nil
			case "second-match":
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
					RouteOptions: option.DNSRouteActionOptions{Server: "first-upstream"},
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
					RouteOptions: option.DNSRouteActionOptions{Server: "second-upstream"},
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
					RouteOptions: option.DNSRouteActionOptions{Server: "first-match"},
				},
			},
		},
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					MatchResponse:  true,
					ResponseAnswer: badoption.Listable[option.DNSRecordOptions]{mustRecord(t, "example.com. IN A 2.2.2.2")},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "second-match"},
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

func TestExchangeLegacyDNSModeDisabledRespondReturnsEvaluatedResponse(t *testing.T) {
	t.Parallel()

	var exchanges []string
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
					Action: C.RuleActionTypeRespond,
				},
			},
		},
	}, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default":  defaultTransport,
			"upstream": &fakeDNSTransport{tag: "upstream", transportType: C.DNSTypeUDP},
		},
	}, &fakeDNSClient{
		exchange: func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
			exchanges = append(exchanges, transport.Tag())
			require.Equal(t, "upstream", transport.Tag())
			return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("1.1.1.1")}, 60), nil
		},
	})
	require.False(t, router.legacyDNSMode)

	response, err := router.Exchange(context.Background(), &mDNS.Msg{
		Question: []mDNS.Question{fixedQuestion("example.com", mDNS.TypeA)},
	}, adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, []string{"upstream"}, exchanges)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("1.1.1.1")}, MessageToAddresses(response))
}

func TestLookupLegacyDNSModeDisabledRespondReturnsEvaluatedResponse(t *testing.T) {
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
					Action: C.RuleActionTypeRespond,
				},
			},
		},
	}, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default":  defaultTransport,
			"upstream": &fakeDNSTransport{tag: "upstream", transportType: C.DNSTypeUDP},
		},
	}, &fakeDNSClient{
		exchange: func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
			require.Equal(t, "upstream", transport.Tag())
			switch message.Question[0].Qtype {
			case mDNS.TypeA:
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("1.1.1.1")}, 60), nil
			case mDNS.TypeAAAA:
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("2001:db8::1")}, 60), nil
			default:
				return nil, E.New("unexpected qtype")
			}
		},
	})
	require.False(t, router.legacyDNSMode)

	addresses, err := router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{
		netip.MustParseAddr("1.1.1.1"),
		netip.MustParseAddr("2001:db8::1"),
	}, addresses)
}

func TestExchangeLegacyDNSModeDisabledRespondWithoutEvaluatedResponseReturnsError(t *testing.T) {
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
					Action: C.RuleActionTypeRespond,
				},
			},
		},
	}, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default":  defaultTransport,
			"upstream": &fakeDNSTransport{tag: "upstream", transportType: C.DNSTypeUDP},
		},
	}, &fakeDNSClient{
		exchange: func(transport adapter.DNSTransport, _ *mDNS.Msg) (*mDNS.Msg, error) {
			require.Equal(t, "upstream", transport.Tag())
			return nil, E.New("upstream exchange failed")
		},
	})
	require.False(t, router.legacyDNSMode)

	response, err := router.Exchange(context.Background(), &mDNS.Msg{
		Question: []mDNS.Question{fixedQuestion("example.com", mDNS.TypeA)},
	}, adapter.DNSQueryOptions{})
	require.Nil(t, response)
	require.ErrorContains(t, err, dnsRespondMissingResponseMessage)
}

func TestLookupLegacyDNSModeDisabledAllowsPartialSuccessForExchangeFailure(t *testing.T) {
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
	router.legacyDNSMode = false

	addresses, err := router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("1.1.1.1")}, addresses)
}

func TestLookupLegacyDNSModeDisabledAllowsPartialSuccessForRcodeError(t *testing.T) {
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
				return &mDNS.Msg{
					MsgHdr: mDNS.MsgHdr{
						Response: true,
						Rcode:    mDNS.RcodeNameError,
					},
					Question: []mDNS.Question{message.Question[0]},
				}, nil
			default:
				return nil, E.New("unexpected qtype")
			}
		},
	})
	router.legacyDNSMode = false

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
	router.legacyDNSMode = false

	addresses, err := router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("2.2.2.2")}, addresses)
}

func TestExchangeLegacyDNSModeDisabledAllowsRouteFakeIPRule(t *testing.T) {
	t.Parallel()

	fakeTransport := &fakeDNSTransport{tag: "fake", transportType: C.DNSTypeFakeIP}
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
		defaultTransport: &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		transports: map[string]adapter.DNSTransport{
			"default": &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
			"fake":    fakeTransport,
		},
	}, &fakeDNSClient{
		exchange: func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
			require.Same(t, fakeTransport, transport)
			return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("198.18.0.1")}, 60), nil
		},
	})

	response, err := router.Exchange(context.Background(), &mDNS.Msg{
		Question: []mDNS.Question{fixedQuestion("example.com", mDNS.TypeA)},
	}, adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("198.18.0.1")}, MessageToAddresses(response))
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
	require.ErrorContains(t, err, "strategy")
	require.ErrorContains(t, err, "deprecated")
}

func TestInitializeRejectsEvaluateFakeIPServerInDefaultRule(t *testing.T) {
	t.Parallel()

	router := &Router{
		ctx:                   context.Background(),
		logger:                log.NewNOPFactory().NewLogger("dns"),
		transport:             &fakeDNSTransportManager{transports: map[string]adapter.DNSTransport{"fake": &fakeDNSTransport{tag: "fake", transportType: C.DNSTypeFakeIP}}},
		client:                &fakeDNSClient{},
		rawRules:              make([]option.DNSRule, 0, 1),
		defaultDomainStrategy: C.DomainStrategyAsIS,
	}
	err := router.Initialize([]option.DNSRule{{
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
	}})
	require.ErrorContains(t, err, "evaluate action cannot use fakeip server")
	require.ErrorContains(t, err, "fake")
}

func TestInitializeRejectsEvaluateFakeIPServerInLogicalRule(t *testing.T) {
	t.Parallel()

	router := &Router{
		ctx:                   context.Background(),
		logger:                log.NewNOPFactory().NewLogger("dns"),
		transport:             &fakeDNSTransportManager{transports: map[string]adapter.DNSTransport{"fake": &fakeDNSTransport{tag: "fake", transportType: C.DNSTypeFakeIP}}},
		client:                &fakeDNSClient{},
		rawRules:              make([]option.DNSRule, 0, 1),
		defaultDomainStrategy: C.DomainStrategyAsIS,
	}
	err := router.Initialize([]option.DNSRule{{
		Type: C.RuleTypeLogical,
		LogicalOptions: option.LogicalDNSRule{
			RawLogicalDNSRule: option.RawLogicalDNSRule{
				Mode: C.LogicalTypeOr,
				Rules: []option.DNSRule{{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultDNSRule{
						RawDefaultDNSRule: option.RawDefaultDNSRule{
							Domain: badoption.Listable[string]{"example.com"},
						},
					},
				}},
			},
			DNSRuleAction: option.DNSRuleAction{
				Action:       C.RuleActionTypeEvaluate,
				RouteOptions: option.DNSRouteActionOptions{Server: "fake"},
			},
		},
	}})
	require.ErrorContains(t, err, "evaluate action cannot use fakeip server")
	require.ErrorContains(t, err, "fake")
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
	require.ErrorContains(t, err, "strategy")
	require.ErrorContains(t, err, "deprecated")
}

func TestInitializeRejectsDNSMatchResponseWithoutPrecedingEvaluate(t *testing.T) {
	t.Parallel()

	router := &Router{
		ctx:                   context.Background(),
		logger:                log.NewNOPFactory().NewLogger("dns"),
		transport:             &fakeDNSTransportManager{},
		client:                &fakeDNSClient{},
		rawRules:              make([]option.DNSRule, 0, 1),
		defaultDomainStrategy: C.DomainStrategyAsIS,
	}
	err := router.Initialize([]option.DNSRule{{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				MatchResponse:  true,
				ResponseAnswer: badoption.Listable[option.DNSRecordOptions]{mustRecord(t, "example.com. IN A 1.1.1.1")},
			},
			DNSRuleAction: option.DNSRuleAction{
				Action:       C.RuleActionTypeRoute,
				RouteOptions: option.DNSRouteActionOptions{Server: "default"},
			},
		},
	}})
	require.ErrorContains(t, err, "preceding evaluate action")
}

func TestInitializeRejectsDNSRespondWithoutPrecedingEvaluate(t *testing.T) {
	t.Parallel()

	router := &Router{
		ctx:                   context.Background(),
		logger:                log.NewNOPFactory().NewLogger("dns"),
		transport:             &fakeDNSTransportManager{},
		client:                &fakeDNSClient{},
		rawRules:              make([]option.DNSRule, 0, 1),
		defaultDomainStrategy: C.DomainStrategyAsIS,
	}
	err := router.Initialize([]option.DNSRule{{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				Domain: badoption.Listable[string]{"example.com"},
			},
			DNSRuleAction: option.DNSRuleAction{
				Action: C.RuleActionTypeRespond,
			},
		},
	}})
	require.ErrorContains(t, err, "preceding evaluate action")
}

func TestInitializeRejectsLogicalDNSRespondWithoutPrecedingEvaluate(t *testing.T) {
	t.Parallel()

	router := &Router{
		ctx:                   context.Background(),
		logger:                log.NewNOPFactory().NewLogger("dns"),
		transport:             &fakeDNSTransportManager{},
		client:                &fakeDNSClient{},
		rawRules:              make([]option.DNSRule, 0, 1),
		defaultDomainStrategy: C.DomainStrategyAsIS,
	}
	err := router.Initialize([]option.DNSRule{{
		Type: C.RuleTypeLogical,
		LogicalOptions: option.LogicalDNSRule{
			RawLogicalDNSRule: option.RawLogicalDNSRule{
				Mode: C.LogicalTypeOr,
				Rules: []option.DNSRule{{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultDNSRule{
						RawDefaultDNSRule: option.RawDefaultDNSRule{
							Domain: badoption.Listable[string]{"example.com"},
						},
					},
				}},
			},
			DNSRuleAction: option.DNSRuleAction{
				Action: C.RuleActionTypeRespond,
			},
		},
	}})
	require.ErrorContains(t, err, "preceding evaluate action")
}

func TestInitializeRejectsEvaluateRuleWithResponseMatchWithoutPrecedingEvaluate(t *testing.T) {
	t.Parallel()

	router := &Router{
		ctx:                   context.Background(),
		logger:                log.NewNOPFactory().NewLogger("dns"),
		transport:             &fakeDNSTransportManager{},
		client:                &fakeDNSClient{},
		rawRules:              make([]option.DNSRule, 0, 1),
		defaultDomainStrategy: C.DomainStrategyAsIS,
	}
	err := router.Initialize([]option.DNSRule{{
		Type: C.RuleTypeLogical,
		LogicalOptions: option.LogicalDNSRule{
			RawLogicalDNSRule: option.RawLogicalDNSRule{
				Mode: C.LogicalTypeOr,
				Rules: []option.DNSRule{
					{
						Type: C.RuleTypeDefault,
						DefaultOptions: option.DefaultDNSRule{
							RawDefaultDNSRule: option.RawDefaultDNSRule{
								Domain: badoption.Listable[string]{"example.com"},
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
						},
					},
				},
			},
			DNSRuleAction: option.DNSRuleAction{
				Action:       C.RuleActionTypeEvaluate,
				RouteOptions: option.DNSRouteActionOptions{Server: "default"},
			},
		},
	}})
	require.ErrorContains(t, err, "preceding evaluate action")
}

func TestInitializeAllowsEvaluateRuleWithResponseMatchAfterPrecedingEvaluate(t *testing.T) {
	t.Parallel()

	router := &Router{
		ctx:                   context.Background(),
		logger:                log.NewNOPFactory().NewLogger("dns"),
		transport:             &fakeDNSTransportManager{},
		client:                &fakeDNSClient{},
		rawRules:              make([]option.DNSRule, 0, 2),
		defaultDomainStrategy: C.DomainStrategyAsIS,
	}
	err := router.Initialize([]option.DNSRule{
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					Domain: badoption.Listable[string]{"bootstrap.example"},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeEvaluate,
					RouteOptions: option.DNSRouteActionOptions{Server: "bootstrap"},
				},
			},
		},
		{
			Type: C.RuleTypeLogical,
			LogicalOptions: option.LogicalDNSRule{
				RawLogicalDNSRule: option.RawLogicalDNSRule{
					Mode: C.LogicalTypeOr,
					Rules: []option.DNSRule{
						{
							Type: C.RuleTypeDefault,
							DefaultOptions: option.DefaultDNSRule{
								RawDefaultDNSRule: option.RawDefaultDNSRule{
									Domain: badoption.Listable[string]{"example.com"},
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
							},
						},
					},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeEvaluate,
					RouteOptions: option.DNSRouteActionOptions{Server: "default"},
				},
			},
		},
	})
	require.NoError(t, err)
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
	require.False(t, router.legacyDNSMode)

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
	require.False(t, router.legacyDNSMode)

	response, err := router.Exchange(context.Background(), &mDNS.Msg{
		Question: []mDNS.Question{fixedQuestion("example.com", mDNS.TypeA)},
	}, adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, mDNS.RcodeRefused, response.Rcode)
	require.Equal(t, []mDNS.Question{fixedQuestion("example.com", mDNS.TypeA)}, response.Question)
}

func TestExchangeLegacyDNSModeDisabledReturnsDropErrorForRejectDropAction(t *testing.T) {
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
						Method: C.RuleActionRejectMethodDrop,
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
	require.False(t, router.legacyDNSMode)

	response, err := router.Exchange(context.Background(), &mDNS.Msg{
		Question: []mDNS.Question{fixedQuestion("example.com", mDNS.TypeA)},
	}, adapter.DNSQueryOptions{})
	require.Nil(t, response)
	require.ErrorIs(t, err, tun.ErrDrop)
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
	require.False(t, router.legacyDNSMode)

	addresses, err := router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{
		netip.MustParseAddr("1.1.1.1"),
		netip.MustParseAddr("2001:db8::1"),
	}, addresses)
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
