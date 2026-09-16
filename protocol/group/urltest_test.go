package group

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/urltest"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"

	"github.com/stretchr/testify/require"
)

func TestNewBandwidthTestOptionsDisabled(t *testing.T) {
	t.Parallel()
	parsed, err := newBandwidthTestOptions(nil, C.DefaultURLTestInterval)
	require.NoError(t, err)
	require.Nil(t, parsed)

	parsed, err = newBandwidthTestOptions(&option.URLTestBandwidthTestOptions{
		URL: "https://example.com/payload",
	}, C.DefaultURLTestInterval)
	require.NoError(t, err)
	require.Nil(t, parsed, "an unset enabled flag must leave the feature inert")
}

func TestNewBandwidthTestOptionsDefaults(t *testing.T) {
	t.Parallel()
	parsed, err := newBandwidthTestOptions(&option.URLTestBandwidthTestOptions{
		Enabled: true,
		URL:     "https://example.com/payload",
	}, C.DefaultURLTestInterval)
	require.NoError(t, err)
	require.NotNil(t, parsed)
	require.Equal(t, uint32(defaultBandwidthMaxBytes), parsed.maxBytes)
	require.Equal(t, C.DefaultURLTestBandwidthTimeout, parsed.timeout)
	require.Equal(t, defaultBandwidthConcurrency, parsed.concurrency)
	require.Equal(t, uint16(defaultBandwidthTolerance), parsed.throughputTolerance)
	require.Equal(t, defaultBandwidthSamples, parsed.samples)
	// Latency ranking stays the default even once measurement is on.
	require.Equal(t, C.URLTestStrategyLatency, parsed.strategy)
	require.Zero(t, parsed.latencyFloor)
	// The throughput cadence derives from the latency cadence rather than matching it.
	require.Equal(t, C.DefaultURLTestInterval*defaultBandwidthIntervalMultiplier, parsed.interval)
}

func TestNewBandwidthTestOptionsDefaultURL(t *testing.T) {
	t.Parallel()
	parsed, err := newBandwidthTestOptions(&option.URLTestBandwidthTestOptions{
		Enabled: true,
	}, C.DefaultURLTestInterval)
	require.NoError(t, err)
	require.Equal(t, C.DefaultURLTestBandwidthURLPrefix+"262144", parsed.link)

	// The default endpoint is sized to the cap, so raising max_bytes must raise the
	// request too rather than leaving the probe short of its own limit.
	parsed, err = newBandwidthTestOptions(&option.URLTestBandwidthTestOptions{
		Enabled:  true,
		MaxBytes: 512 * 1024,
	}, C.DefaultURLTestInterval)
	require.NoError(t, err)
	require.Equal(t, C.DefaultURLTestBandwidthURLPrefix+"524288", parsed.link)

	// An explicit URL is never overridden.
	parsed, err = newBandwidthTestOptions(&option.URLTestBandwidthTestOptions{
		Enabled: true,
		URL:     "https://example.com/payload",
	}, C.DefaultURLTestInterval)
	require.NoError(t, err)
	require.Equal(t, "https://example.com/payload", parsed.link)
}

func TestNewBandwidthTestOptionsValidation(t *testing.T) {
	t.Parallel()
	for name, options := range map[string]*option.URLTestBandwidthTestOptions{
		"max bytes above cap":  {MaxBytes: maxBandwidthMaxBytes + 1},
		"unknown strategy":     {Strategy: "fastest"},
		"negative timeout":     {Timeout: badoption.Duration(-time.Second)},
		"negative interval":    {Interval: badoption.Duration(-time.Second)},
		"negative concurrency": {Concurrency: -1},
		"negative samples":     {Samples: -1},
	} {
		t.Run(name, func(t *testing.T) {
			options.Enabled = true
			options.URL = "https://example.com/payload"
			_, err := newBandwidthTestOptions(options, C.DefaultURLTestInterval)
			require.Error(t, err)
		})
	}
}

func TestNewBandwidthTestOptionsLatencyFloor(t *testing.T) {
	t.Parallel()
	parsed, err := newBandwidthTestOptions(&option.URLTestBandwidthTestOptions{
		Enabled:      true,
		URL:          "https://example.com/payload",
		Strategy:     C.URLTestStrategyThroughputWithLatencyFloor,
		LatencyFloor: badoption.Duration(400 * time.Millisecond),
	}, C.DefaultURLTestInterval)
	require.NoError(t, err)
	require.Equal(t, uint16(400), parsed.latencyFloor)

	// Delay is stored as uint16 milliseconds, so a larger floor is clamped rather
	// than wrapping around into an absurdly small one.
	parsed, err = newBandwidthTestOptions(&option.URLTestBandwidthTestOptions{
		Enabled:      true,
		URL:          "https://example.com/payload",
		Strategy:     C.URLTestStrategyThroughputWithLatencyFloor,
		LatencyFloor: badoption.Duration(2 * time.Minute),
	}, C.DefaultURLTestInterval)
	require.NoError(t, err)
	require.Equal(t, uint16(65535), parsed.latencyFloor)
}

func TestMedianThroughput(t *testing.T) {
	t.Parallel()
	require.Zero(t, medianThroughput(nil))
	require.Equal(t, uint32(100), medianThroughput([]uint32{100}))
	require.Equal(t, uint32(150), medianThroughput([]uint32{100, 200}))
	require.Equal(t, uint32(200), medianThroughput([]uint32{100, 200, 300}))
	// The point of smoothing: one transient failure among good samples must not
	// evict an otherwise healthy outbound.
	require.Equal(t, uint32(900), medianThroughput([]uint32{1000, 0, 900}))
	// Sustained failure does decay it out of contention.
	require.Zero(t, medianThroughput([]uint32{1000, 0, 0}))
}

func TestBeatsIncumbent(t *testing.T) {
	t.Parallel()
	// A challenger inside the tolerance band leaves the incumbent in place, so the
	// group does not flap and repeatedly interrupt live connections.
	require.False(t, beatsIncumbent(1200, 1000, 25))
	require.False(t, beatsIncumbent(1250, 1000, 25))
	require.True(t, beatsIncumbent(1251, 1000, 25))
	require.True(t, beatsIncumbent(1001, 1000, 0))
	require.False(t, beatsIncumbent(1000, 1000, 0))
}

func TestRecordBandwidthWindow(t *testing.T) {
	t.Parallel()
	group := &URLTestGroup{
		bandwidth:        &bandwidthTestOptions{samples: 3},
		bandwidthHistory: make(map[string]*bandwidthState),
	}
	epoch := group.currentBandwidthEpoch()
	record := func(throughput uint32) uint32 {
		smoothed, recorded := group.recordBandwidthChecked(epoch, "proxy", throughput)
		require.True(t, recorded)
		return smoothed
	}
	require.Equal(t, uint32(100), record(100))
	require.Equal(t, uint32(150), record(200))
	require.Equal(t, uint32(200), record(300))
	// The window slides rather than growing, so old samples stop counting.
	require.Equal(t, uint32(300), record(400))
	require.Len(t, group.bandwidthHistory["proxy"].samples, 3)
	require.Equal(t, uint32(300), group.loadBandwidth("proxy"))

	group.resetBandwidth()
	require.Zero(t, group.loadBandwidth("proxy"))
	// A probe that started before the reset measured the previous network path;
	// its sample must be dropped instead of written back into the cleared history.
	_, recorded := group.recordBandwidthChecked(epoch, "proxy", 500)
	require.False(t, recorded)
	require.Zero(t, group.loadBandwidth("proxy"))
	// A probe started after the reset records normally.
	newEpoch := group.currentBandwidthEpoch()
	require.Equal(t, epoch+1, newEpoch)
	smoothed, recorded := group.recordBandwidthChecked(newEpoch, "proxy", 600)
	require.True(t, recorded)
	require.Equal(t, uint32(600), smoothed)
}

// hangingDialOutbound reproduces the #4255 failure mode: a dialer that parks
// forever while ignoring the context deadline it was handed.
type hangingDialOutbound struct {
	outbound.Adapter
	block    chan struct{}
	release  sync.Once
	dials    atomic.Int32
	returned atomic.Int32
}

func (o *hangingDialOutbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	o.dials.Add(1)
	<-o.block
	o.returned.Add(1)
	return nil, E.New("dial released")
}

func (o *hangingDialOutbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, E.New("not implemented")
}

// unblock lets parked dials return; safe to call more than once.
func (o *hangingDialOutbound) unblock() {
	o.release.Do(func() { close(o.block) })
}

type staticOutboundManager struct {
	outbounds []adapter.Outbound
}

func (m *staticOutboundManager) Start(stage adapter.StartStage) error { return nil }
func (m *staticOutboundManager) Close() error                         { return nil }
func (m *staticOutboundManager) Outbounds() []adapter.Outbound        { return m.outbounds }
func (m *staticOutboundManager) Default() adapter.Outbound            { return m.outbounds[0] }
func (m *staticOutboundManager) Remove(tag string) error              { return nil }
func (m *staticOutboundManager) Outbound(tag string) (adapter.Outbound, bool) {
	for _, detour := range m.outbounds {
		if detour.Tag() == tag {
			return detour, true
		}
	}
	return nil, false
}

func (m *staticOutboundManager) Create(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, outboundType string, options any) error {
	return nil
}

func newHangingBandwidthGroup(t *testing.T) (*URLTestGroup, *hangingDialOutbound) {
	hang := &hangingDialOutbound{
		Adapter: outbound.NewAdapter(C.TypeDirect, "hang", []string{N.NetworkTCP, N.NetworkUDP}, nil),
		block:   make(chan struct{}),
	}
	ctx := service.ContextWithPtr[urltest.HistoryStorage](context.Background(), urltest.NewHistoryStorage())
	group, err := NewURLTestGroup(
		ctx, &staticOutboundManager{outbounds: []adapter.Outbound{hang}},
		log.NewNOPFactory().Logger(), []adapter.Outbound{hang},
		"https://example.com/payload", 50*time.Millisecond, 0, time.Second, false,
		&option.URLTestBandwidthTestOptions{
			Enabled: true,
			URL:     "https://example.com/payload",
			Timeout: badoption.Duration(50 * time.Millisecond),
		},
	)
	require.NoError(t, err)
	t.Cleanup(hang.unblock)
	return group, hang
}

func TestBandwidthRoundCeilingReleasesGuard(t *testing.T) {
	t.Parallel()
	group, hang := newHangingBandwidthGroup(t)

	// One worker and default concurrency give a budget of two probe timeouts; the
	// dial ignores its deadline, so the round must be abandoned at the budget
	// instead of wedging on batch.Wait() forever.
	done := make(chan struct{})
	go func() {
		group.bandwidthTest(context.Background(), true)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("bandwidth round did not return under its budget")
	}

	// The re-entrancy guard went down with the round: a second forced round spawns
	// a worker and dials again instead of being silently skipped.
	group.bandwidthTest(context.Background(), true)
	require.Eventually(t, func() bool {
		return hang.dials.Load() == 2
	}, 2*time.Second, 5*time.Millisecond)
}

func TestBandwidthAbandonedSampleDroppedAfterReset(t *testing.T) {
	t.Parallel()
	group, hang := newHangingBandwidthGroup(t)

	// A round is abandoned while its worker hangs, then the network changes.
	group.bandwidthTest(context.Background(), true)
	require.Eventually(t, func() bool {
		return hang.dials.Load() == 1
	}, 2*time.Second, 5*time.Millisecond)
	group.resetBandwidth()

	// When the abandoned worker finally unblocks, its sample belongs to the
	// previous network path and must not surface in the cleared history.
	hang.unblock()
	require.Eventually(t, func() bool {
		return hang.returned.Load() == 1
	}, 2*time.Second, 5*time.Millisecond)
	// The dial has returned; give the worker a moment to run its record path,
	// then require that no sample survived the reset.
	time.Sleep(100 * time.Millisecond)
	require.Empty(t, group.bandwidthHistory)
}
