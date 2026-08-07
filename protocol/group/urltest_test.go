package group

import (
	"testing"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json/badoption"

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
	require.Equal(t, uint32(100), group.recordBandwidth("proxy", 100))
	require.Equal(t, uint32(150), group.recordBandwidth("proxy", 200))
	require.Equal(t, uint32(200), group.recordBandwidth("proxy", 300))
	// The window slides rather than growing, so old samples stop counting.
	require.Equal(t, uint32(300), group.recordBandwidth("proxy", 400))
	require.Len(t, group.bandwidthHistory["proxy"].samples, 3)
	require.Equal(t, uint32(300), group.loadBandwidth("proxy"))

	group.resetBandwidth()
	require.Zero(t, group.loadBandwidth("proxy"))
}
