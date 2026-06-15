package group

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/urltest"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/stretchr/testify/require"
)

type mockOutbound struct {
	tag     string
	network []string
}

func (m *mockOutbound) Type() string           { return "mock" }
func (m *mockOutbound) Tag() string            { return m.tag }
func (m *mockOutbound) Network() []string      { return m.network }
func (m *mockOutbound) Dependencies() []string { return nil }

func (m *mockOutbound) DialContext(context.Context, string, M.Socksaddr) (net.Conn, error) {
	return nil, E.New("mock dial")
}

func (m *mockOutbound) ListenPacket(context.Context, M.Socksaddr) (net.PacketConn, error) {
	return nil, E.New("mock listen")
}

func tcpOutbound(tag string) adapter.Outbound {
	return &mockOutbound{tag: tag, network: []string{N.NetworkTCP, N.NetworkUDP}}
}

func newTestGroup(tiers [][]adapter.Outbound, tolerance uint16) *PriorityGroup {
	outbounds := make([]adapter.Outbound, 0)
	for _, tier := range tiers {
		outbounds = append(outbounds, tier...)
	}
	return &PriorityGroup{
		tiers:          tiers,
		outbounds:      outbounds,
		history:        urltest.NewHistoryStorage(),
		tolerance:      tolerance,
		tierUpChecks:   2,
		tierDownChecks: 1,
		stateTCP:       tierState{active: -1, healthyStreak: make([]uint16, len(tiers))},
		stateUDP:       tierState{active: -1, healthyStreak: make([]uint16, len(tiers))},
	}
}

func (g *PriorityGroup) setDelay(tag string, delay uint16) {
	g.history.StoreURLTestHistory(tag, &adapter.URLTestHistory{Time: time.Now(), Delay: delay})
}

func TestDecideTier(t *testing.T) {
	t.Parallel()

	type round struct {
		alive      []bool
		wantActive int
	}
	testCases := []struct {
		name           string
		tiers          int
		tierUpChecks   uint16
		tierDownChecks uint16
		rounds         []round
	}{
		{
			name:           "cold start picks highest live tier",
			tiers:          3,
			tierUpChecks:   2,
			tierDownChecks: 1,
			rounds: []round{
				{alive: []bool{false, true, false}, wantActive: 1},
			},
		},
		{
			name:           "hold active tier while alive",
			tiers:          3,
			tierUpChecks:   2,
			tierDownChecks: 1,
			rounds: []round{
				{alive: []bool{true, true, true}, wantActive: 0},
				{alive: []bool{true, true, true}, wantActive: 0},
				{alive: []bool{true, false, true}, wantActive: 0},
			},
		},
		{
			name:           "drop immediately on full death with down_checks=1",
			tiers:          3,
			tierUpChecks:   2,
			tierDownChecks: 1,
			rounds: []round{
				{alive: []bool{true, true, true}, wantActive: 0},
				{alive: []bool{false, true, true}, wantActive: 1},
			},
		},
		{
			name:           "drop is delayed by down_checks=2",
			tiers:          3,
			tierUpChecks:   2,
			tierDownChecks: 2,
			rounds: []round{
				{alive: []bool{true, true, true}, wantActive: 0},
				{alive: []bool{false, true, true}, wantActive: 0},
				{alive: []bool{false, true, true}, wantActive: 1},
			},
		},
		{
			name:           "climb back only after up_checks consecutive healthy rounds",
			tiers:          3,
			tierUpChecks:   2,
			tierDownChecks: 1,
			rounds: []round{
				{alive: []bool{false, false, true}, wantActive: 2},
				{alive: []bool{true, false, true}, wantActive: 2},
				{alive: []bool{true, false, true}, wantActive: 0},
			},
		},
		{
			name:           "climb resets when the higher tier flaps",
			tiers:          3,
			tierUpChecks:   2,
			tierDownChecks: 1,
			rounds: []round{
				{alive: []bool{false, false, true}, wantActive: 2},
				{alive: []bool{true, false, true}, wantActive: 2},
				{alive: []bool{false, false, true}, wantActive: 2},
				{alive: []bool{true, false, true}, wantActive: 2},
				{alive: []bool{true, false, true}, wantActive: 0},
			},
		},
		{
			name:           "climb picks highest tier meeting the streak",
			tiers:          3,
			tierUpChecks:   2,
			tierDownChecks: 1,
			rounds: []round{
				{alive: []bool{false, false, true}, wantActive: 2},
				{alive: []bool{false, true, true}, wantActive: 2},
				{alive: []bool{true, true, true}, wantActive: 1},
			},
		},
		{
			name:           "hold active tier when nothing is reachable",
			tiers:          3,
			tierUpChecks:   2,
			tierDownChecks: 1,
			rounds: []round{
				{alive: []bool{true, true, true}, wantActive: 0},
				{alive: []bool{false, false, false}, wantActive: 0},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			state := &tierState{active: -1, healthyStreak: make([]uint16, testCase.tiers)}
			for roundIndex, r := range testCase.rounds {
				decideTier(state, r.alive, testCase.tierUpChecks, testCase.tierDownChecks)
				require.Equalf(t, r.wantActive, state.active, "round %d", roundIndex)
			}
		})
	}
}

func TestNewPriorityValidation(t *testing.T) {
	t.Parallel()
	logger := log.NewNOPFactory().Logger()
	testCases := []struct {
		name    string
		tiers   [][]string
		wantErr string
	}{
		{name: "valid", tiers: [][]string{{"a", "b"}, {"c"}}, wantErr: ""},
		{name: "missing tiers", tiers: nil, wantErr: "missing tiers"},
		{name: "empty tiers slice", tiers: [][]string{}, wantErr: "missing tiers"},
		{name: "empty tier", tiers: [][]string{{"a"}, {}}, wantErr: "empty tier 1"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewPriority(context.Background(), nil, logger, "p", option.PriorityOutboundOptions{Tiers: testCase.tiers})
			if testCase.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, testCase.wantErr)
			}
		})
	}
}

func TestPriorityGroupValidation(t *testing.T) {
	t.Parallel()
	logger := log.NewNOPFactory().Logger()
	// interval must be <= idle_timeout
	_, err := NewPriorityGroup(context.Background(), nil, logger, nil, "", 10*time.Minute, 0, time.Minute, 0, 0, false)
	require.ErrorContains(t, err, "interval must be less or equal than idle_timeout")
	// a history storage must be present in the context
	_, err = NewPriorityGroup(context.Background(), nil, logger, nil, "", 0, 0, 0, 0, 0, false)
	require.ErrorContains(t, err, "missing URL test history storage")
}

func TestTierAlive(t *testing.T) {
	t.Parallel()
	g := newTestGroup([][]adapter.Outbound{
		{tcpOutbound("a"), tcpOutbound("b")},
		{tcpOutbound("c")},
	}, 50)
	require.False(t, g.tierAlive(0, N.NetworkTCP), "no history -> dead")
	g.setDelay("b", 100)
	require.True(t, g.tierAlive(0, N.NetworkTCP), "one live member -> alive")
	require.False(t, g.tierAlive(1, N.NetworkTCP), "tier with no history -> dead")
	// a UDP-only probe still counts because mock supports both networks
	require.True(t, g.tierAlive(0, N.NetworkUDP))
}

func TestSelectInTierLowestDelay(t *testing.T) {
	t.Parallel()
	g := newTestGroup([][]adapter.Outbound{
		{tcpOutbound("a"), tcpOutbound("b"), tcpOutbound("c")},
	}, 50)
	g.setDelay("a", 300)
	g.setDelay("b", 60)
	g.setDelay("c", 120)
	selected := g.selectInTier(0, N.NetworkTCP, nil)
	require.NotNil(t, selected)
	require.Equal(t, "b", selected.Tag(), "picks lowest-delay member")
}

func TestSelectInTierToleranceHysteresis(t *testing.T) {
	t.Parallel()
	g := newTestGroup([][]adapter.Outbound{
		{tcpOutbound("a"), tcpOutbound("b")},
	}, 50)
	current := g.tiers[0][0] // "a"
	g.setDelay("a", 100)
	g.setDelay("b", 80) // only 20ms better, within tolerance 50 -> keep current
	selected := g.selectInTier(0, N.NetworkTCP, current)
	require.Equal(t, "a", selected.Tag(), "stays on current within tolerance")
	g.setDelay("b", 30) // now 70ms better, beyond tolerance -> switch
	selected = g.selectInTier(0, N.NetworkTCP, current)
	require.Equal(t, "b", selected.Tag(), "switches when beyond tolerance")
}

func TestSelectInTierSkipsDeadMember(t *testing.T) {
	t.Parallel()
	g := newTestGroup([][]adapter.Outbound{
		{tcpOutbound("a"), tcpOutbound("b")},
	}, 50)
	g.setDelay("a", 200) // b has no history -> dead
	selected := g.selectInTier(0, N.NetworkTCP, nil)
	require.Equal(t, "a", selected.Tag())
}

func TestSelectFallback(t *testing.T) {
	t.Parallel()
	g := newTestGroup([][]adapter.Outbound{
		{tcpOutbound("a")},
		{tcpOutbound("b")},
	}, 50)
	// no history anywhere -> first network-supporting outbound
	require.Equal(t, "a", g.Select(N.NetworkTCP).Tag())
	// history on lower tier only -> that becomes the live pick
	g.setDelay("b", 50)
	require.Equal(t, "b", g.Select(N.NetworkTCP).Tag())
}

func TestUpdateTierEndToEnd(t *testing.T) {
	t.Parallel()
	g := newTestGroup([][]adapter.Outbound{
		{tcpOutbound("a")}, // tier 0
		{tcpOutbound("b")}, // tier 1
		{tcpOutbound("c")}, // tier 2
	}, 50)

	// All alive: hold tier 0 even though tier 2 is faster (priority over RTT).
	g.setDelay("a", 300)
	g.setDelay("b", 100)
	g.setDelay("c", 20)
	require.Equal(t, "a", g.updateTier(&g.stateTCP, N.NetworkTCP).Tag())

	// Tier 0 dies -> drop to the best live lower tier (down_checks=1).
	g.history.DeleteURLTestHistory("a")
	require.Equal(t, "b", g.updateTier(&g.stateTCP, N.NetworkTCP).Tag())

	// Tier 0 recovers: needs tier_up_checks=2 consecutive healthy rounds.
	g.setDelay("a", 300)
	require.Equal(t, "b", g.updateTier(&g.stateTCP, N.NetworkTCP).Tag(), "round 1: not yet")
	require.Equal(t, "a", g.updateTier(&g.stateTCP, N.NetworkTCP).Tag(), "round 2: climb back")
}

func TestUpdateTierIndependentTCPUDP(t *testing.T) {
	t.Parallel()
	g := newTestGroup([][]adapter.Outbound{
		{tcpOutbound("a")},
		{tcpOutbound("b")},
	}, 50)
	g.setDelay("a", 100)
	g.setDelay("b", 100)
	// TCP holds tier 0.
	require.Equal(t, "a", g.updateTier(&g.stateTCP, N.NetworkTCP).Tag())
	require.Equal(t, 0, g.stateTCP.active)
	// UDP state is independent and starts cold, also resolving to tier 0.
	require.Equal(t, "a", g.updateTier(&g.stateUDP, N.NetworkUDP).Tag())
	require.Equal(t, 0, g.stateUDP.active)
	// Drop only tier 0's TCP view by deleting history; both networks share the
	// same history here, so both drop — but the states advance independently.
	g.history.DeleteURLTestHistory("a")
	require.Equal(t, "b", g.updateTier(&g.stateTCP, N.NetworkTCP).Tag())
	require.Equal(t, 1, g.stateTCP.active)
	require.Equal(t, 0, g.stateUDP.active, "udp state not advanced until its own round runs")
}
