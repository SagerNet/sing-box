package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/protocol/group"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Each test gets its own ports. Reusing a fixed set across sequentially started
// instances is flaky: a closing instance can still answer on a port the next test has
// just bound, and its probes then fail against a backend that is already gone.
var urlTestPortCursor atomic.Uint32

type urlTestEnv struct {
	clientPort  uint16
	serverPortA uint16
	serverPortB uint16
	clashPort   uint16
}

func newURLTestEnv() urlTestEnv {
	base := uint16(19000 + urlTestPortCursor.Add(10))
	return urlTestEnv{
		clientPort:  base,
		serverPortA: base + 1,
		serverPortB: base + 2,
		clashPort:   base + 3,
	}
}

const (
	// The probe URLs point at a placeholder destination; each proxy server rewrites
	// it to its own backend with a route action, which is what gives the two paths
	// independently controllable characteristics.
	urlTestLatencyURL   = "http://127.0.0.1:1/latency"
	urlTestBandwidthURL = "http://127.0.0.1:1/payload"

	urlTestProbeInterval  = 500 * time.Millisecond
	urlTestMaxProbeBytes  = 64 * 1024
	urlTestPayloadChunk   = 8 * 1024
	urlTestPayloadChunks  = 64 // 512 KiB available, well above the probe cap
	urlTestEventually     = 15 * time.Second
	urlTestEventuallyTick = 100 * time.Millisecond

	// Both delays are non-zero on purpose. Select treats a delay of 0 as "no
	// incumbent yet" (protocol/group/urltest.go, minDelay == 0), and a loopback probe
	// really can measure 0 ms, which lets the slower outbound override the faster
	// one. Keep every backend above a millisecond.
	urlTestFastDelay = 50 * time.Millisecond
	urlTestSlowDelay = 350 * time.Millisecond
)

// payloadServer stands in for the endpoint a urltest group probes. Its time to
// response headers and its transfer rate are controlled separately, because the whole
// premise of bandwidth-aware selection is that those two properties decouple: a shaped
// path answers quickly and then crawls.
type payloadServer struct {
	server      *httptest.Server
	headerDelay atomic.Int64
	chunkDelay  atomic.Int64
}

func startPayloadServer(t *testing.T, headerDelay time.Duration, chunkDelay time.Duration) *payloadServer {
	s := new(payloadServer)
	s.headerDelay.Store(int64(headerDelay))
	s.chunkDelay.Store(int64(chunkDelay))
	s.server = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.server.Close)
	return s
}

func (s *payloadServer) handle(w http.ResponseWriter, r *http.Request) {
	if delay := time.Duration(s.headerDelay.Load()); delay > 0 {
		time.Sleep(delay)
	}
	if r.URL.Path == "/latency" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	flusher, canFlush := w.(http.Flusher)
	chunk := make([]byte, urlTestPayloadChunk)
	chunkDelay := time.Duration(s.chunkDelay.Load())
	for range urlTestPayloadChunks {
		if _, err := w.Write(chunk); err != nil {
			return
		}
		if canFlush {
			flusher.Flush()
		}
		if chunkDelay > 0 {
			time.Sleep(chunkDelay)
		}
	}
}

func (s *payloadServer) port(t *testing.T) uint16 {
	_, portString, err := net.SplitHostPort(s.server.Listener.Addr().String())
	require.NoError(t, err)
	port, err := strconv.ParseUint(portString, 10, 16)
	require.NoError(t, err)
	return uint16(port)
}

func (s *payloadServer) setHeaderDelay(delay time.Duration) {
	s.headerDelay.Store(int64(delay))
}

// urlTestOptions builds a self-contained topology: two shadowsocks servers in the same
// instance, each routing the probe to its own backend, and a urltest group over the two
// matching outbounds.
func urlTestOptions(t *testing.T, env urlTestEnv, backendA *payloadServer, backendB *payloadServer, bandwidth *option.URLTestBandwidthTestOptions, clashAPI bool) option.Options {
	options := option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.AddrFrom4([4]byte{127, 0, 0, 1}))),
						ListenPort: env.clientPort,
					},
				},
			},
			{
				Type: C.TypeShadowsocks,
				Tag:  "ss-in-a",
				Options: &option.ShadowsocksInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.AddrFrom4([4]byte{127, 0, 0, 1}))),
						ListenPort: env.serverPortA,
					},
					Method: "none",
				},
			},
			{
				Type: C.TypeShadowsocks,
				Tag:  "ss-in-b",
				Options: &option.ShadowsocksInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.AddrFrom4([4]byte{127, 0, 0, 1}))),
						ListenPort: env.serverPortB,
					},
					Method: "none",
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
				Tag:  "direct",
			},
			{
				Type: C.TypeShadowsocks,
				Tag:  "proxy-a",
				Options: &option.ShadowsocksOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: env.serverPortA,
					},
					Method: "none",
				},
			},
			{
				Type: C.TypeShadowsocks,
				Tag:  "proxy-b",
				Options: &option.ShadowsocksOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: env.serverPortB,
					},
					Method: "none",
				},
			},
			{
				Type: C.TypeURLTest,
				Tag:  "auto",
				Options: &option.URLTestOutboundOptions{
					Outbounds:     []string{"proxy-a", "proxy-b"},
					URL:           urlTestLatencyURL,
					Interval:      badoption.Duration(urlTestProbeInterval),
					IdleTimeout:   badoption.Duration(time.Minute),
					BandwidthTest: bandwidth,
				},
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				urlTestRouteRule("mixed-in", "auto", 0),
				urlTestRouteRule("ss-in-a", "direct", backendA.port(t)),
				urlTestRouteRule("ss-in-b", "direct", backendB.port(t)),
			},
		},
	}
	if clashAPI {
		options.Experimental = &option.ExperimentalOptions{
			ClashAPI: &option.ClashAPIOptions{
				ExternalController: "127.0.0.1:" + strconv.Itoa(int(env.clashPort)),
			},
		}
	}
	return options
}

func urlTestRouteRule(inbound string, outbound string, overridePort uint16) option.Rule {
	action := option.RouteActionOptions{
		Outbound: outbound,
	}
	if overridePort != 0 {
		action.OverrideAddress = "127.0.0.1"
		action.OverridePort = overridePort
	}
	return option.Rule{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultRule{
			RawDefaultRule: option.RawDefaultRule{
				Inbound: []string{inbound},
			},
			RuleAction: option.RuleAction{
				Action:       C.RuleActionTypeRoute,
				RouteOptions: action,
			},
		},
	}
}

func urlTestOutbound(t *testing.T, instance *box.Box) *group.URLTest {
	t.Helper()
	outbound, loaded := instance.Outbound().Outbound("auto")
	require.True(t, loaded, "urltest outbound not found")
	urlTest, isURLTest := outbound.(*group.URLTest)
	require.True(t, isURLTest, "outbound is not a urltest group")
	return urlTest
}

func selectedOutbound(t *testing.T, instance *box.Box) string {
	t.Helper()
	return urlTestOutbound(t, instance).Now()
}

// touchGroup sends one connection through the group, which is what starts its probe
// tickers: an untouched group stays suspended and never probes.
func touchGroup(t *testing.T, env urlTestEnv, backend *payloadServer) {
	t.Helper()
	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", env.clientPort), socks.Version5, "", "")
	conn, err := dialer.DialContext(context.Background(), N.NetworkTCP, M.ParseSocksaddrHostPort("127.0.0.1", backend.port(t)))
	if err == nil {
		conn.Close()
	}
}

func requireSelected(t *testing.T, instance *box.Box, expected string) {
	t.Helper()
	var last string
	if !assert.Eventually(t, func() bool {
		last = selectedOutbound(t, instance)
		return last == expected
	}, urlTestEventually, urlTestEventuallyTick) {
		// Eventually evaluates its message eagerly, so report the last observed
		// value separately rather than the one from before polling started.
		require.FailNowf(t, "selection did not converge", "expected %s, last saw %q", expected, last)
	}
}

// TestURLTestSelectsLowerLatency is the baseline the group has always promised, and
// which had no coverage before this file.
func TestURLTestSelectsLowerLatency(t *testing.T) {
	env := newURLTestEnv()
	backendFast := startPayloadServer(t, urlTestFastDelay, 0)
	backendSlow := startPayloadServer(t, urlTestSlowDelay, 0)
	instance := startInstance(t, urlTestOptions(t, env, backendFast, backendSlow, nil, false))
	// Touch the group so its ticker runs. An untouched group probes once at
	// PostStart and never retries, which makes a single transient failure permanent.
	touchGroup(t, env, backendFast)
	requireSelected(t, instance, "proxy-a")
}

// TestURLTestDropsFailedOutbound covers the failure path: a probe error deletes the
// history entry, which must move selection to the surviving outbound.
func TestURLTestDropsFailedOutbound(t *testing.T) {
	env := newURLTestEnv()
	backendFast := startPayloadServer(t, urlTestFastDelay, 0)
	backendSlow := startPayloadServer(t, urlTestSlowDelay, 0)
	instance := startInstance(t, urlTestOptions(t, env, backendFast, backendSlow, nil, false))
	// Touch the group so its ticker runs. An untouched group probes once at
	// PostStart and never retries, which makes a single transient failure permanent.
	touchGroup(t, env, backendFast)
	requireSelected(t, instance, "proxy-a")

	// Take the selected path's backend away; its probe now fails while the other
	// still answers.
	backendFast.server.Close()
	urlTestOutbound(t, instance).CheckOutbounds()
	requireSelected(t, instance, "proxy-b")
}

// TestURLTestToleranceHoldsIncumbent guards against the group flapping between two
// outbounds whose latencies are close, which would churn connections.
func TestURLTestToleranceHoldsIncumbent(t *testing.T) {
	env := newURLTestEnv()
	backendA := startPayloadServer(t, urlTestFastDelay, 0)
	backendB := startPayloadServer(t, urlTestSlowDelay, 0)
	options := urlTestOptions(t, env, backendA, backendB, nil, false)
	options.Outbounds[3].Options.(*option.URLTestOutboundOptions).Tolerance = 400
	instance := startInstance(t, options)
	touchGroup(t, env, backendA)
	requireSelected(t, instance, "proxy-a")

	// Make B faster, but by less than the tolerance. The incumbent must hold.
	backendA.setHeaderDelay(250 * time.Millisecond)
	backendB.setHeaderDelay(urlTestFastDelay)
	for range 3 {
		urlTestOutbound(t, instance).CheckOutbounds()
	}
	require.Equal(t, "proxy-a", selectedOutbound(t, instance))
}

func TestURLTestReportsAllDelays(t *testing.T) {
	env := newURLTestEnv()
	backendA := startPayloadServer(t, urlTestFastDelay, 0)
	backendB := startPayloadServer(t, 150*time.Millisecond, 0)
	instance := startInstance(t, urlTestOptions(t, env, backendA, backendB, nil, false))

	// URLTest reports only the outbounds it actually re-probed: an entry newer than
	// the interval is skipped, and PostStart has just populated both. Wait it out so
	// the sweep has something to do.
	time.Sleep(urlTestProbeInterval + 200*time.Millisecond)

	result, err := urlTestOutbound(t, instance).URLTest(context.Background())
	require.NoError(t, err)
	require.Contains(t, result, "proxy-a")
	require.Contains(t, result, "proxy-b")
}

// TestURLTestBandwidthMeasuresThroughput checks the metric end to end, through a real
// encrypted proxy, and asserts it reaches the Clash API where clients can read it.
func TestURLTestBandwidthMeasuresThroughput(t *testing.T) {
	env := newURLTestEnv()
	backendA := startPayloadServer(t, urlTestFastDelay, 0)
	backendB := startPayloadServer(t, urlTestFastDelay, 0)
	instance := startInstance(t, urlTestOptions(t, env, backendA, backendB, &option.URLTestBandwidthTestOptions{
		Enabled:  true,
		URL:      urlTestBandwidthURL,
		MaxBytes: urlTestMaxProbeBytes,
		Interval: badoption.Duration(urlTestProbeInterval),
		Timeout:  badoption.Duration(10 * time.Second),
	}, true))
	touchGroup(t, env, backendA)

	require.Eventually(t, func() bool {
		history := clashProxyHistory(t, env, "proxy-a")
		return history != nil && history.Throughput > 0 && history.Bytes > 0
	}, urlTestEventually, urlTestEventuallyTick, "throughput never appeared on the Clash API")

	history := clashProxyHistory(t, env, "proxy-a")
	// The cap is a hard limit on what a single probe reads.
	require.LessOrEqual(t, history.Bytes, uint32(urlTestMaxProbeBytes))
	// Latency must survive alongside it rather than being overwritten.
	require.NotZero(t, history.Time)
	// Measuring throughput must not disturb selection under the default strategy.
	require.NotEmpty(t, selectedOutbound(t, instance))
}

// TestURLTestBandwidthStrategyRanking is the issue's own table made executable: the two
// paths disagree about which is better depending on which property you measure, and the
// strategy decides who wins.
func TestURLTestBandwidthStrategyRanking(t *testing.T) {
	for _, testCase := range []struct {
		strategy string
		expected string
	}{
		// proxy-a answers fastest but then crawls.
		{C.URLTestStrategyLatency, "proxy-a"},
		// proxy-b is slower to first byte but actually moves data.
		{C.URLTestStrategyThroughput, "proxy-b"},
	} {
		t.Run(testCase.strategy, func(t *testing.T) {
			env := newURLTestEnv()
			shaped := startPayloadServer(t, urlTestFastDelay, 40*time.Millisecond)
			quick := startPayloadServer(t, 250*time.Millisecond, 0)
			instance := startInstance(t, urlTestOptions(t, env, shaped, quick, &option.URLTestBandwidthTestOptions{
				Enabled:  true,
				URL:      urlTestBandwidthURL,
				MaxBytes: urlTestMaxProbeBytes,
				Interval: badoption.Duration(urlTestProbeInterval),
				Timeout:  badoption.Duration(20 * time.Second),
				Strategy: testCase.strategy,
				// One sample, so the assertion does not wait for a median to fill.
				Samples: 1,
			}, false))
			touchGroup(t, env, shaped)
			requireSelected(t, instance, testCase.expected)
		})
	}
}

func clashProxyHistory(t *testing.T, env urlTestEnv, tag string) *adapter.URLTestHistory {
	t.Helper()
	response, err := http.Get("http://127.0.0.1:" + strconv.Itoa(int(env.clashPort)) + "/proxies/" + tag)
	if err != nil {
		return nil
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil
	}
	var proxy struct {
		History []*adapter.URLTestHistory `json:"history"`
	}
	if json.NewDecoder(response.Body).Decode(&proxy) != nil || len(proxy.History) == 0 {
		return nil
	}
	return proxy.History[0]
}
