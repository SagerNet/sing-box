package dns

import (
	"context"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	R "github.com/sagernet/sing-box/route/rule"

	mDNS "github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

type fakeDNSTransport struct {
	tag         string
	delay       time.Duration
	rcode       int
	address     netip.Addr
	exchangeErr error

	access       sync.Mutex
	queryCount   atomic.Int32
	firstQueried time.Time
}

func (t *fakeDNSTransport) Start(stage adapter.StartStage) error {
	return nil
}

func (t *fakeDNSTransport) Close() error {
	return nil
}

func (t *fakeDNSTransport) Type() string {
	return "fake"
}

func (t *fakeDNSTransport) Tag() string {
	return t.tag
}

func (t *fakeDNSTransport) Dependencies() []string {
	return nil
}

func (t *fakeDNSTransport) Reset() {
}

func (t *fakeDNSTransport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	t.access.Lock()
	if t.firstQueried.IsZero() {
		t.firstQueried = time.Now()
	}
	t.access.Unlock()
	t.queryCount.Add(1)
	select {
	case <-time.After(t.delay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if t.exchangeErr != nil {
		return nil, t.exchangeErr
	}
	if t.rcode != mDNS.RcodeSuccess {
		return FixedResponseStatus(message, t.rcode), nil
	}
	return FixedResponse(message.Id, message.Question[0], []netip.Addr{t.address}, 300), nil
}

func (t *fakeDNSTransport) ExchangeAsync(ctx context.Context, message *mDNS.Msg, callback func(response *mDNS.Msg, err error)) {
	go func() {
		callback(t.Exchange(ctx, message))
	}()
}

type fakeDNSTransportManager struct {
	transports       map[string]adapter.DNSTransport
	defaultTransport adapter.DNSTransport
}

func (m *fakeDNSTransportManager) Start(stage adapter.StartStage) error {
	return nil
}

func (m *fakeDNSTransportManager) Close() error {
	return nil
}

func (m *fakeDNSTransportManager) Transports() []adapter.DNSTransport {
	return nil
}

func (m *fakeDNSTransportManager) Transport(tag string) (adapter.DNSTransport, bool) {
	transport, loaded := m.transports[tag]
	return transport, loaded
}

func (m *fakeDNSTransportManager) Default() adapter.DNSTransport {
	return m.defaultTransport
}

func (m *fakeDNSTransportManager) FakeIP() adapter.FakeIPTransport {
	return nil
}

func (m *fakeDNSTransportManager) Remove(tag string) error {
	return nil
}

func (m *fakeDNSTransportManager) Create(ctx context.Context, logger log.ContextLogger, tag string, outboundType string, options any) error {
	return nil
}

func raceTestRouter(t *testing.T, transports ...*fakeDNSTransport) *Router {
	transportMap := make(map[string]adapter.DNSTransport)
	for _, transport := range transports {
		transportMap[transport.tag] = transport
	}
	return &Router{
		ctx:    context.Background(),
		logger: log.NewNOPFactory().Logger(),
		transport: &fakeDNSTransportManager{
			transports:       transportMap,
			defaultTransport: transportMap["final"],
		},
		client: NewClient(ClientOptions{
			Context:      context.Background(),
			DisableCache: true,
			Logger:       log.NewNOPFactory().Logger(),
		}),
	}
}

func raceTestRules(t *testing.T, rawRules []option.DNSRule) []adapter.DNSRule {
	rules := make([]adapter.DNSRule, 0, len(rawRules))
	for _, rawRule := range rawRules {
		rule, err := R.NewDNSRule(context.Background(), log.NewNOPFactory().Logger(), rawRule, true, false)
		require.NoError(t, err)
		rules = append(rules, rule)
	}
	return rules
}

func raceTestExchange(router *Router, rules []adapter.DNSRule) exchangeWithRulesResult {
	message := &mDNS.Msg{
		MsgHdr: mDNS.MsgHdr{
			Id:               1,
			RecursionDesired: true,
		},
		Question: []mDNS.Question{{
			Name:   "race.example.org.",
			Qtype:  mDNS.TypeA,
			Qclass: mDNS.ClassINET,
		}},
	}
	metadata := &adapter.InboundContext{
		Domain:    "race.example.org",
		QueryType: mDNS.TypeA,
	}
	ctx := adapter.WithContext(context.Background(), metadata)
	return router.exchangeWithRules(ctx, rules, message, adapter.DNSQueryOptions{}, false)
}

func evaluateRule(server string, tag string, speculative bool) option.DNSRule {
	return option.DNSRule{
		Type: "",
		DefaultOptions: option.DefaultDNSRule{
			DNSRuleAction: option.DNSRuleAction{
				Action: C.RuleActionTypeEvaluate,
				RouteOptions: option.DNSRouteActionOptions{
					Server:      server,
					Tag:         tag,
					Speculative: speculative,
				},
			},
		},
	}
}

func respondRule(responseTag string, race bool, requireSuccess bool) option.DNSRule {
	rule := option.DNSRule{
		Type: "",
		DefaultOptions: option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				MatchResponse: &option.DNSRuleMatchResponse{Enabled: true, Tag: responseTag},
			},
			DNSRuleAction: option.DNSRuleAction{
				Action: C.RuleActionTypeRespond,
				Race:   race,
			},
		},
	}
	if requireSuccess {
		successRcode := option.DNSRCode(mDNS.RcodeSuccess)
		rule.DefaultOptions.ResponseRcode = &successRcode
	}
	return rule
}

func routeRule(server string, speculative bool) option.DNSRule {
	return option.DNSRule{
		Type: "",
		DefaultOptions: option.DefaultDNSRule{
			DNSRuleAction: option.DNSRuleAction{
				Action: C.RuleActionTypeRoute,
				RouteOptions: option.DNSRouteActionOptions{
					Server:      server,
					Speculative: speculative,
				},
			},
		},
	}
}

func responseAddress(t *testing.T, response *mDNS.Msg) netip.Addr {
	require.NotNil(t, response)
	require.Len(t, response.Answer, 1)
	record, isA := response.Answer[0].(*mDNS.A)
	require.True(t, isA)
	address, _ := netip.AddrFromSlice(record.A)
	return address.Unmap()
}

// Both evaluate queries must launch in parallel, and a failed primary must
// fall through to the secondary instead of failing the request.
func TestDNSEvaluateParallelFallback(t *testing.T) {
	t.Parallel()
	transportX := &fakeDNSTransport{tag: "x", delay: 200 * time.Millisecond, exchangeErr: context.DeadlineExceeded}
	transportY := &fakeDNSTransport{tag: "y", delay: 10 * time.Millisecond, rcode: mDNS.RcodeSuccess, address: netip.MustParseAddr("192.0.2.2")}
	router := raceTestRouter(t, transportX, transportY)
	rules := raceTestRules(t, []option.DNSRule{
		evaluateRule("x", "x", false),
		evaluateRule("y", "y", false),
		respondRule("x", false, true),
		respondRule("y", false, true),
	})
	startTime := time.Now()
	result := raceTestExchange(router, rules)
	require.NoError(t, result.err)
	require.Equal(t, netip.MustParseAddr("192.0.2.2"), responseAddress(t, result.response))
	require.Equal(t, int32(1), transportX.queryCount.Load())
	require.Equal(t, int32(1), transportY.queryCount.Load())
	require.Less(t, transportY.firstQueried.Sub(transportX.firstQueried), 100*time.Millisecond)
	require.Less(t, time.Since(startTime), 350*time.Millisecond)
}

// The first race rule whose response arrives and matches must commit
// immediately, without waiting for the slower rule written before it.
func TestDNSRaceFastestWins(t *testing.T) {
	t.Parallel()
	transportX := &fakeDNSTransport{tag: "x", delay: 500 * time.Millisecond, rcode: mDNS.RcodeSuccess, address: netip.MustParseAddr("192.0.2.1")}
	transportY := &fakeDNSTransport{tag: "y", delay: 20 * time.Millisecond, rcode: mDNS.RcodeSuccess, address: netip.MustParseAddr("192.0.2.2")}
	router := raceTestRouter(t, transportX, transportY)
	rules := raceTestRules(t, []option.DNSRule{
		evaluateRule("x", "x", false),
		evaluateRule("y", "y", false),
		respondRule("x", true, true),
		respondRule("y", true, true),
	})
	startTime := time.Now()
	result := raceTestExchange(router, rules)
	require.NoError(t, result.err)
	require.Equal(t, netip.MustParseAddr("192.0.2.2"), responseAddress(t, result.response))
	require.Less(t, time.Since(startTime), 400*time.Millisecond)
}

// Without race, rule order decides even when a later response arrives first.
func TestDNSOrderedReadsPreferEarlierRule(t *testing.T) {
	t.Parallel()
	transportX := &fakeDNSTransport{tag: "x", delay: 200 * time.Millisecond, rcode: mDNS.RcodeSuccess, address: netip.MustParseAddr("192.0.2.1")}
	transportY := &fakeDNSTransport{tag: "y", delay: 10 * time.Millisecond, rcode: mDNS.RcodeSuccess, address: netip.MustParseAddr("192.0.2.2")}
	router := raceTestRouter(t, transportX, transportY)
	rules := raceTestRules(t, []option.DNSRule{
		evaluateRule("x", "x", false),
		evaluateRule("y", "y", false),
		respondRule("x", false, true),
		respondRule("y", false, true),
	})
	result := raceTestExchange(router, rules)
	require.NoError(t, result.err)
	require.Equal(t, netip.MustParseAddr("192.0.2.1"), responseAddress(t, result.response))
}

// A pending race rule must hold back the default route: the default server
// is never queried when the race rule hits, and is queried only after the
// race decision resolved when it misses.
func TestDNSRaceBarrierProtectsDefaultRoute(t *testing.T) {
	t.Parallel()
	transportHit := &fakeDNSTransport{tag: "x", delay: 100 * time.Millisecond, rcode: mDNS.RcodeSuccess, address: netip.MustParseAddr("192.0.2.1")}
	transportFinal := &fakeDNSTransport{tag: "final", delay: 10 * time.Millisecond, rcode: mDNS.RcodeSuccess, address: netip.MustParseAddr("192.0.2.9")}
	router := raceTestRouter(t, transportHit, transportFinal)
	rules := raceTestRules(t, []option.DNSRule{
		evaluateRule("x", "x", false),
		respondRule("x", true, true),
	})
	result := raceTestExchange(router, rules)
	require.NoError(t, result.err)
	require.Equal(t, netip.MustParseAddr("192.0.2.1"), responseAddress(t, result.response))
	require.Equal(t, int32(0), transportFinal.queryCount.Load())

	transportMiss := &fakeDNSTransport{tag: "x", delay: 100 * time.Millisecond, rcode: mDNS.RcodeNameError}
	transportFinal = &fakeDNSTransport{tag: "final", delay: 10 * time.Millisecond, rcode: mDNS.RcodeSuccess, address: netip.MustParseAddr("192.0.2.9")}
	router = raceTestRouter(t, transportMiss, transportFinal)
	rules = raceTestRules(t, []option.DNSRule{
		evaluateRule("x", "x", false),
		respondRule("x", true, true),
	})
	startTime := time.Now()
	result = raceTestExchange(router, rules)
	require.NoError(t, result.err)
	require.Equal(t, netip.MustParseAddr("192.0.2.9"), responseAddress(t, result.response))
	require.Equal(t, int32(1), transportFinal.queryCount.Load())
	require.GreaterOrEqual(t, transportFinal.firstQueried.Sub(startTime), 90*time.Millisecond)
}

// A speculative route launches while the race decision is pending, but its
// response is only used after the race rule missed.
func TestDNSSpeculativeRoute(t *testing.T) {
	t.Parallel()
	transportMiss := &fakeDNSTransport{tag: "x", delay: 100 * time.Millisecond, rcode: mDNS.RcodeNameError}
	transportFinal := &fakeDNSTransport{tag: "final", delay: 10 * time.Millisecond, rcode: mDNS.RcodeSuccess, address: netip.MustParseAddr("192.0.2.9")}
	router := raceTestRouter(t, transportMiss, transportFinal)
	rules := raceTestRules(t, []option.DNSRule{
		evaluateRule("x", "x", false),
		respondRule("x", true, true),
		routeRule("final", true),
	})
	startTime := time.Now()
	result := raceTestExchange(router, rules)
	require.NoError(t, result.err)
	require.Equal(t, netip.MustParseAddr("192.0.2.9"), responseAddress(t, result.response))
	require.Equal(t, int32(1), transportFinal.queryCount.Load())
	require.Less(t, transportFinal.firstQueried.Sub(startTime), 90*time.Millisecond)
	require.GreaterOrEqual(t, time.Since(startTime), 90*time.Millisecond)

	transportHit := &fakeDNSTransport{tag: "x", delay: 100 * time.Millisecond, rcode: mDNS.RcodeSuccess, address: netip.MustParseAddr("192.0.2.1")}
	transportFinal = &fakeDNSTransport{tag: "final", delay: 10 * time.Millisecond, rcode: mDNS.RcodeSuccess, address: netip.MustParseAddr("192.0.2.9")}
	router = raceTestRouter(t, transportHit, transportFinal)
	rules = raceTestRules(t, []option.DNSRule{
		evaluateRule("x", "x", false),
		respondRule("x", true, true),
		routeRule("final", true),
	})
	result = raceTestExchange(router, rules)
	require.NoError(t, result.err)
	require.Equal(t, netip.MustParseAddr("192.0.2.1"), responseAddress(t, result.response))
	require.Equal(t, int32(1), transportFinal.queryCount.Load())
}

// A matched rule without race must not take effect while a race rule is
// still pending: a race hit wins even when the other rule matched earlier,
// and on a race miss the other rule takes effect only after that decision.
func TestDNSNonRaceCommitWaitsForPendingRace(t *testing.T) {
	t.Parallel()
	transportHit := &fakeDNSTransport{tag: "x", delay: 150 * time.Millisecond, rcode: mDNS.RcodeSuccess, address: netip.MustParseAddr("192.0.2.1")}
	transportY := &fakeDNSTransport{tag: "y", delay: 10 * time.Millisecond, rcode: mDNS.RcodeSuccess, address: netip.MustParseAddr("192.0.2.2")}
	router := raceTestRouter(t, transportHit, transportY)
	rules := raceTestRules(t, []option.DNSRule{
		evaluateRule("x", "x", false),
		evaluateRule("y", "y", false),
		respondRule("x", true, true),
		respondRule("y", false, true),
	})
	startTime := time.Now()
	result := raceTestExchange(router, rules)
	require.NoError(t, result.err)
	require.Equal(t, netip.MustParseAddr("192.0.2.1"), responseAddress(t, result.response))
	require.GreaterOrEqual(t, time.Since(startTime), 140*time.Millisecond)

	transportMiss := &fakeDNSTransport{tag: "x", delay: 150 * time.Millisecond, rcode: mDNS.RcodeNameError}
	transportY = &fakeDNSTransport{tag: "y", delay: 10 * time.Millisecond, rcode: mDNS.RcodeSuccess, address: netip.MustParseAddr("192.0.2.2")}
	router = raceTestRouter(t, transportMiss, transportY)
	rules = raceTestRules(t, []option.DNSRule{
		evaluateRule("x", "x", false),
		evaluateRule("y", "y", false),
		respondRule("x", true, true),
		respondRule("y", false, true),
	})
	startTime = time.Now()
	result = raceTestExchange(router, rules)
	require.NoError(t, result.err)
	require.Equal(t, netip.MustParseAddr("192.0.2.2"), responseAddress(t, result.response))
	require.GreaterOrEqual(t, time.Since(startTime), 140*time.Millisecond)
}

// speculative on a route rule with match_response launches the route query as
// soon as the rule matched, while its response is only used after the pending
// race rule missed.
func TestDNSSpeculativeRouteOnBindingRule(t *testing.T) {
	t.Parallel()
	transportMiss := &fakeDNSTransport{tag: "x", delay: 100 * time.Millisecond, rcode: mDNS.RcodeNameError}
	transportY := &fakeDNSTransport{tag: "y", delay: 10 * time.Millisecond, rcode: mDNS.RcodeSuccess, address: netip.MustParseAddr("192.0.2.2")}
	transportFinal := &fakeDNSTransport{tag: "final", delay: 10 * time.Millisecond, rcode: mDNS.RcodeSuccess, address: netip.MustParseAddr("192.0.2.9")}
	router := raceTestRouter(t, transportMiss, transportY, transportFinal)
	successRcode := option.DNSRCode(mDNS.RcodeSuccess)
	boundRouteRule := option.DNSRule{
		Type: "",
		DefaultOptions: option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				MatchResponse: &option.DNSRuleMatchResponse{Enabled: true, Tag: "y"},
				ResponseRcode: &successRcode,
			},
			DNSRuleAction: option.DNSRuleAction{
				Action: C.RuleActionTypeRoute,
				RouteOptions: option.DNSRouteActionOptions{
					Server:      "final",
					Speculative: true,
				},
			},
		},
	}
	rules := raceTestRules(t, []option.DNSRule{
		evaluateRule("x", "x", false),
		evaluateRule("y", "y", false),
		respondRule("x", true, true),
		boundRouteRule,
	})
	startTime := time.Now()
	result := raceTestExchange(router, rules)
	require.NoError(t, result.err)
	require.Equal(t, netip.MustParseAddr("192.0.2.9"), responseAddress(t, result.response))
	require.Equal(t, int32(1), transportFinal.queryCount.Load())
	require.Less(t, transportFinal.firstQueried.Sub(startTime), 90*time.Millisecond)
	require.GreaterOrEqual(t, time.Since(startTime), 90*time.Millisecond)
}

// A race rule that rejects its response (NXDOMAIN vs required success)
// disarms and lets the other race rule win.
func TestDNSRaceSkipsRejectedResponse(t *testing.T) {
	t.Parallel()
	transportX := &fakeDNSTransport{tag: "x", delay: 10 * time.Millisecond, rcode: mDNS.RcodeNameError}
	transportY := &fakeDNSTransport{tag: "y", delay: 100 * time.Millisecond, rcode: mDNS.RcodeSuccess, address: netip.MustParseAddr("192.0.2.2")}
	router := raceTestRouter(t, transportX, transportY)
	rules := raceTestRules(t, []option.DNSRule{
		evaluateRule("x", "x", false),
		evaluateRule("y", "y", false),
		respondRule("x", true, true),
		respondRule("y", true, true),
	})
	result := raceTestExchange(router, rules)
	require.NoError(t, result.err)
	require.Equal(t, netip.MustParseAddr("192.0.2.2"), responseAddress(t, result.response))
}

// A logical race rule is judged once all of its referenced responses arrived:
// it wins over a slower race rule when its sub-rules match, and on a miss the
// slower race rule takes over.
func TestDNSLogicalRace(t *testing.T) {
	t.Parallel()
	successRcode := option.DNSRCode(mDNS.RcodeSuccess)
	logicalRule := func() option.DNSRule {
		return option.DNSRule{
			Type: C.RuleTypeLogical,
			LogicalOptions: option.LogicalDNSRule{
				RawLogicalDNSRule: option.RawLogicalDNSRule{
					Mode: C.LogicalTypeAnd,
					Rules: []option.DNSRule{
						{
							Type: C.RuleTypeDefault,
							DefaultOptions: option.DefaultDNSRule{
								RawDefaultDNSRule: option.RawDefaultDNSRule{
									MatchResponse: &option.DNSRuleMatchResponse{Enabled: true},
									ResponseRcode: &successRcode,
								},
							},
						},
						{
							Type: C.RuleTypeDefault,
							DefaultOptions: option.DefaultDNSRule{
								RawDefaultDNSRule: option.RawDefaultDNSRule{
									MatchResponse: &option.DNSRuleMatchResponse{Enabled: true, Tag: "y"},
									ResponseRcode: &successRcode,
								},
							},
						},
					},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action: C.RuleActionTypeRespond,
					Race:   true,
				},
			},
		}
	}

	transportX := &fakeDNSTransport{tag: "x", delay: 100 * time.Millisecond, rcode: mDNS.RcodeSuccess, address: netip.MustParseAddr("192.0.2.1")}
	transportY := &fakeDNSTransport{tag: "y", delay: 10 * time.Millisecond, rcode: mDNS.RcodeSuccess, address: netip.MustParseAddr("192.0.2.2")}
	transportZ := &fakeDNSTransport{tag: "z", delay: 250 * time.Millisecond, rcode: mDNS.RcodeSuccess, address: netip.MustParseAddr("192.0.2.3")}
	router := raceTestRouter(t, transportX, transportY, transportZ)
	rules := raceTestRules(t, []option.DNSRule{
		evaluateRule("x", "", false),
		evaluateRule("y", "y", false),
		evaluateRule("z", "z", false),
		logicalRule(),
		respondRule("z", true, true),
	})
	startTime := time.Now()
	result := raceTestExchange(router, rules)
	require.NoError(t, result.err)
	require.Equal(t, netip.MustParseAddr("192.0.2.1"), responseAddress(t, result.response))
	require.GreaterOrEqual(t, time.Since(startTime), 90*time.Millisecond)
	require.Less(t, time.Since(startTime), 240*time.Millisecond)

	transportX = &fakeDNSTransport{tag: "x", delay: 100 * time.Millisecond, rcode: mDNS.RcodeNameError}
	transportY = &fakeDNSTransport{tag: "y", delay: 10 * time.Millisecond, rcode: mDNS.RcodeSuccess, address: netip.MustParseAddr("192.0.2.2")}
	transportZ = &fakeDNSTransport{tag: "z", delay: 250 * time.Millisecond, rcode: mDNS.RcodeSuccess, address: netip.MustParseAddr("192.0.2.3")}
	router = raceTestRouter(t, transportX, transportY, transportZ)
	rules = raceTestRules(t, []option.DNSRule{
		evaluateRule("x", "", false),
		evaluateRule("y", "y", false),
		evaluateRule("z", "z", false),
		logicalRule(),
		respondRule("z", true, true),
	})
	startTime = time.Now()
	result = raceTestExchange(router, rules)
	require.NoError(t, result.err)
	require.Equal(t, netip.MustParseAddr("192.0.2.3"), responseAddress(t, result.response))
	require.GreaterOrEqual(t, time.Since(startTime), 240*time.Millisecond)
}
