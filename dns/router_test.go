package dns

import (
	"context"
	"errors"
	"net"
	"net/netip"
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
	return nil, errors.New("unused transport exchange")
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
	return errors.New("unsupported")
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
	ruleSet, loaded := r.ruleSets[tag]
	return ruleSet, loaded
}
func (r *fakeRouter) Rules() []adapter.Rule                      { return nil }
func (r *fakeRouter) NeedFindProcess() bool                      { return false }
func (r *fakeRouter) NeedFindNeighbor() bool                     { return false }
func (r *fakeRouter) NeighborResolver() adapter.NeighborResolver { return nil }
func (r *fakeRouter) AppendTracker(adapter.ConnectionTracker)    {}
func (r *fakeRouter) ResetNetwork()                              {}

type fakeRuleSet struct {
	access    sync.Mutex
	metadata  adapter.RuleSetMetadata
	callbacks list.List[adapter.RuleSetUpdateCallback]
	refs      int
}

func (s *fakeRuleSet) Name() string                                                  { return "fake-rule-set" }
func (s *fakeRuleSet) StartContext(context.Context, *adapter.HTTPStartContext) error { return nil }
func (s *fakeRuleSet) PostStart() error                                              { return nil }
func (s *fakeRuleSet) Metadata() adapter.RuleSetMetadata {
	s.access.Lock()
	defer s.access.Unlock()
	return s.metadata
}
func (s *fakeRuleSet) ExtractIPSet() []*netipx.IPSet { return nil }
func (s *fakeRuleSet) IncRef() {
	s.access.Lock()
	defer s.access.Unlock()
	s.refs++
}

func (s *fakeRuleSet) DecRef() {
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
func (s *fakeRuleSet) Close() error                       { return nil }
func (s *fakeRuleSet) Match(*adapter.InboundContext) bool { return true }
func (s *fakeRuleSet) String() string                     { return "fake-rule-set" }
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
		return nil, errors.New("unused client lookup")
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
	t.Helper()
	router := &Router{
		ctx:                   ctx,
		logger:                log.NewNOPFactory().NewLogger("dns"),
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
		rules:                 make([]adapter.DNSRule, 0, 1),
		defaultDomainStrategy: C.DomainStrategyAsIS,
	}
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
		rules:                 make([]adapter.DNSRule, 0, 2),
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

func TestRuleSetUpdateSetsRuntimeErrorWhenRebuildFails(t *testing.T) {
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

	require.True(t, router.legacyDNSMode)

	fakeSet.updateMetadata(adapter.RuleSetMetadata{
		ContainsDNSQueryTypeRule: true,
	})

	_, err := router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{})
	require.ErrorContains(t, err, "ip_cidr and ip_is_private require match_response")
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

	router.rulesAccess.RLock()
	defer router.rulesAccess.RUnlock()
	require.True(t, router.closing)
	require.Nil(t, router.rules)
	require.Empty(t, router.ruleSetCallbacks)
	require.NoError(t, router.runtimeRuleError)
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
			return nil, nil, errors.New("unexpected transport")
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
	var lookups []string
	client := &fakeDNSClient{
		lookup: func(transport adapter.DNSTransport, domain string, options adapter.DNSQueryOptions) ([]netip.Addr, *mDNS.Msg, error) {
			require.Equal(t, "example.com", domain)
			require.Equal(t, C.DomainStrategyIPv4Only, options.LookupStrategy)
			lookups = append(lookups, transport.Tag())
			switch transport.Tag() {
			case "private":
				response := FixedResponse(0, fixedQuestion(domain, mDNS.TypeA), []netip.Addr{netip.MustParseAddr("8.8.8.8")}, 60)
				return MessageToAddresses(response), response, nil
			case "default":
				response := FixedResponse(0, fixedQuestion(domain, mDNS.TypeA), []netip.Addr{netip.MustParseAddr("9.9.9.9")}, 60)
				return MessageToAddresses(response), response, nil
			}
			return nil, nil, errors.New("unexpected transport")
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
	require.Equal(t, []string{"private", "default"}, lookups)
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
	var lookups []string
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
			lookups = append(lookups, transport.Tag())
			switch transport.Tag() {
			case "private":
				response := FixedResponse(0, fixedQuestion(domain, mDNS.TypeA), []netip.Addr{netip.MustParseAddr("8.8.8.8")}, 60)
				return MessageToAddresses(response), response, nil
			case "default":
				response := FixedResponse(0, fixedQuestion(domain, mDNS.TypeA), []netip.Addr{netip.MustParseAddr("9.9.9.9")}, 60)
				return MessageToAddresses(response), response, nil
			}
			return nil, nil, errors.New("unexpected transport")
		},
	})

	require.True(t, router.legacyDNSMode)

	addresses, err := router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{
		LookupStrategy: C.DomainStrategyIPv4Only,
	})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("9.9.9.9")}, addresses)
	require.Equal(t, []string{"private", "default"}, lookups)
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
				return nil, errors.New("unexpected transport")
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
				return nil, errors.New("unexpected transport")
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
				return nil, errors.New("unexpected transport")
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
				return nil, errors.New("unexpected transport")
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
				return nil, errors.New("unexpected transport")
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
						return nil, errors.New("upstream exchange failed")
					case "selected":
						return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("8.8.8.8")}, 60), nil
					case "default":
						return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("4.4.4.4")}, 60), nil
					default:
						return nil, errors.New("unexpected transport")
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
				return nil, errors.New("ipv6 failed")
			default:
				return nil, errors.New("unexpected qtype")
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
				return nil, errors.New("unexpected transport")
			}
		},
	})
	router.legacyDNSMode = false

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
				return nil, errors.New("unexpected transport")
			}
		},
	})
	require.False(t, router.legacyDNSMode)

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
				return nil, errors.New("unexpected transport")
			}
		},
	})
	require.False(t, router.legacyDNSMode)

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
				return nil, errors.New("unexpected transport")
			}
		},
	})
	require.False(t, router.legacyDNSMode)

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
		rules:                 make([]adapter.DNSRule, 0, 1),
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
		rules:                 make([]adapter.DNSRule, 0, 1),
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

	require.True(t, router.legacyDNSMode)

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

func TestLookupLegacyDNSModeDisabledUsesInputStrategy(t *testing.T) {
	t.Parallel()

	defaultTransport := &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP}
	var qTypes []uint16
	router := newTestRouter(t, nil, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default": defaultTransport,
		},
	}, &fakeDNSClient{
		exchange: func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
			qTypes = append(qTypes, message.Question[0].Qtype)
			if message.Question[0].Qtype == mDNS.TypeA {
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("2.2.2.2")}, 60), nil
			}
			return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("2001:db8::2")}, 60), nil
		},
	})
	router.legacyDNSMode = false

	addresses, err := router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{
		Strategy: C.DomainStrategyIPv4Only,
	})
	require.NoError(t, err)
	require.Equal(t, []uint16{mDNS.TypeA}, qTypes)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("2.2.2.2")}, addresses)
}

func TestLookupLegacyDNSModeDisabledUsesDefaultStrategy(t *testing.T) {
	t.Parallel()

	defaultTransport := &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP}
	var qTypes []uint16
	router := newTestRouter(t, nil, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default": defaultTransport,
		},
	}, &fakeDNSClient{
		exchange: func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
			qTypes = append(qTypes, message.Question[0].Qtype)
			if message.Question[0].Qtype == mDNS.TypeA {
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("2.2.2.2")}, 60), nil
			}
			return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("2001:db8::2")}, 60), nil
		},
	})
	router.defaultDomainStrategy = C.DomainStrategyIPv4Only
	router.legacyDNSMode = false

	addresses, err := router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, []uint16{mDNS.TypeA}, qTypes)
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
				return nil, errors.New("unexpected transport")
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
		rules:                 make([]adapter.DNSRule, 0, 1),
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
		rules:                 make([]adapter.DNSRule, 0, 1),
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
