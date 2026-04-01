package rule

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-tun"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/x/list"

	"github.com/stretchr/testify/require"
	"go4.org/netipx"
)

type ruleSetItemTestRouter struct {
	ruleSets map[string]adapter.RuleSet
}

func (r *ruleSetItemTestRouter) Start(adapter.StartStage) error { return nil }
func (r *ruleSetItemTestRouter) Close() error                   { return nil }
func (r *ruleSetItemTestRouter) PreMatch(adapter.InboundContext, tun.DirectRouteContext, time.Duration, bool) (tun.DirectRouteDestination, error) {
	return nil, nil
}

func (r *ruleSetItemTestRouter) RouteConnection(context.Context, net.Conn, adapter.InboundContext) error {
	return nil
}

func (r *ruleSetItemTestRouter) RoutePacketConnection(context.Context, N.PacketConn, adapter.InboundContext) error {
	return nil
}

func (r *ruleSetItemTestRouter) RouteConnectionEx(context.Context, net.Conn, adapter.InboundContext, N.CloseHandlerFunc) {
}

func (r *ruleSetItemTestRouter) RoutePacketConnectionEx(context.Context, N.PacketConn, adapter.InboundContext, N.CloseHandlerFunc) {
}

func (r *ruleSetItemTestRouter) RuleSet(tag string) (adapter.RuleSet, bool) {
	ruleSet, loaded := r.ruleSets[tag]
	return ruleSet, loaded
}
func (r *ruleSetItemTestRouter) Rules() []adapter.Rule                      { return nil }
func (r *ruleSetItemTestRouter) NeedFindProcess() bool                      { return false }
func (r *ruleSetItemTestRouter) NeedFindNeighbor() bool                     { return false }
func (r *ruleSetItemTestRouter) NeighborResolver() adapter.NeighborResolver { return nil }
func (r *ruleSetItemTestRouter) AppendTracker(adapter.ConnectionTracker)    {}
func (r *ruleSetItemTestRouter) ResetNetwork()                              {}

type countingRuleSet struct {
	name string
	refs atomic.Int32
}

func (s *countingRuleSet) Name() string                                                  { return s.name }
func (s *countingRuleSet) StartContext(context.Context, *adapter.HTTPStartContext) error { return nil }
func (s *countingRuleSet) PostStart() error                                              { return nil }
func (s *countingRuleSet) Metadata() adapter.RuleSetMetadata                             { return adapter.RuleSetMetadata{} }
func (s *countingRuleSet) ExtractIPSet() []*netipx.IPSet                                 { return nil }
func (s *countingRuleSet) IncRef()                                                       { s.refs.Add(1) }
func (s *countingRuleSet) DecRef() {
	if s.refs.Add(-1) < 0 {
		panic("rule-set: negative refs")
	}
}
func (s *countingRuleSet) Cleanup() {}
func (s *countingRuleSet) RegisterCallback(adapter.RuleSetUpdateCallback) *list.Element[adapter.RuleSetUpdateCallback] {
	return nil
}
func (s *countingRuleSet) UnregisterCallback(*list.Element[adapter.RuleSetUpdateCallback]) {}
func (s *countingRuleSet) Close() error                                                    { return nil }
func (s *countingRuleSet) Match(*adapter.InboundContext) bool                              { return true }
func (s *countingRuleSet) String() string                                                  { return s.name }
func (s *countingRuleSet) RefCount() int32                                                 { return s.refs.Load() }

func TestRuleSetItemCloseReleasesRefs(t *testing.T) {
	t.Parallel()

	firstSet := &countingRuleSet{name: "first"}
	secondSet := &countingRuleSet{name: "second"}
	item := NewRuleSetItem(&ruleSetItemTestRouter{
		ruleSets: map[string]adapter.RuleSet{
			"first":  firstSet,
			"second": secondSet,
		},
	}, []string{"first", "second"}, false, false)

	require.NoError(t, item.Start())
	require.EqualValues(t, 1, firstSet.RefCount())
	require.EqualValues(t, 1, secondSet.RefCount())

	require.NoError(t, item.Close())
	require.Zero(t, firstSet.RefCount())
	require.Zero(t, secondSet.RefCount())

	require.NoError(t, item.Close())
	require.Zero(t, firstSet.RefCount())
	require.Zero(t, secondSet.RefCount())
}

func TestRuleSetItemStartRollbackOnFailure(t *testing.T) {
	t.Parallel()

	firstSet := &countingRuleSet{name: "first"}
	item := NewRuleSetItem(&ruleSetItemTestRouter{
		ruleSets: map[string]adapter.RuleSet{
			"first": firstSet,
		},
	}, []string{"first", "missing"}, false, false)

	err := item.Start()
	require.ErrorContains(t, err, "rule-set not found: missing")
	require.Zero(t, firstSet.RefCount())
}

func TestRuleSetItemRestartKeepsBalancedRefs(t *testing.T) {
	t.Parallel()

	firstSet := &countingRuleSet{name: "first"}
	item := NewRuleSetItem(&ruleSetItemTestRouter{
		ruleSets: map[string]adapter.RuleSet{
			"first": firstSet,
		},
	}, []string{"first"}, false, false)

	require.NoError(t, item.Start())
	require.EqualValues(t, 1, firstSet.RefCount())

	require.NoError(t, item.Start())
	require.EqualValues(t, 1, firstSet.RefCount())

	require.NoError(t, item.Close())
	require.Zero(t, firstSet.RefCount())
}
