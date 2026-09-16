package group

import (
	"context"
	"maps"
	"math"
	"net"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/interrupt"
	"github.com/sagernet/sing-box/common/urltest"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/batch"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/pause"
)

func RegisterURLTest(registry *outbound.Registry) {
	outbound.Register[option.URLTestOutboundOptions](registry, C.TypeURLTest, NewURLTest)
}

var (
	_ adapter.OutboundGroup           = (*URLTest)(nil)
	_ adapter.InterfaceUpdateListener = (*URLTest)(nil)
	_ adapter.Referrer                = (*URLTest)(nil)
)

type URLTest struct {
	outbound.Adapter
	ctx                          context.Context
	outbound                     adapter.OutboundManager
	connection                   adapter.ConnectionManager
	logger                       log.ContextLogger
	tags                         []string
	link                         string
	interval                     time.Duration
	tolerance                    uint16
	idleTimeout                  time.Duration
	group                        *URLTestGroup
	checkAccess                  sync.Mutex
	interruptExternalConnections bool
	bandwidthTest                *option.URLTestBandwidthTestOptions
}

func NewURLTest(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.URLTestOutboundOptions) (adapter.Outbound, error) {
	outbound := &URLTest{
		Adapter:                      outbound.NewAdapter(C.TypeURLTest, tag, []string{N.NetworkTCP, N.NetworkUDP}, options.Outbounds),
		ctx:                          ctx,
		outbound:                     service.FromContext[adapter.OutboundManager](ctx),
		connection:                   service.FromContext[adapter.ConnectionManager](ctx),
		logger:                       logger,
		tags:                         options.Outbounds,
		link:                         options.URL,
		interval:                     time.Duration(options.Interval),
		tolerance:                    options.Tolerance,
		idleTimeout:                  time.Duration(options.IdleTimeout),
		interruptExternalConnections: options.InterruptExistConnections,
		bandwidthTest:                options.BandwidthTest,
	}
	if len(outbound.tags) == 0 {
		return nil, E.New("missing tags")
	}
	return outbound, nil
}

func (s *URLTest) Start() error {
	outbounds := make([]adapter.Outbound, 0, len(s.tags))
	for i, tag := range s.tags {
		detour, loaded := s.outbound.Outbound(tag)
		if !loaded {
			return E.New("outbound ", i, " not found: ", tag)
		}
		outbounds = append(outbounds, detour)
	}
	group, err := NewURLTestGroup(s.ctx, s.outbound, s.logger, outbounds, s.link, s.interval, s.tolerance, s.idleTimeout, s.interruptExternalConnections, s.bandwidthTest)
	if err != nil {
		return err
	}
	s.group = group
	return nil
}

func (s *URLTest) PostStart() error {
	s.group.PostStart()
	return nil
}

func (s *URLTest) Close() error {
	return common.Close(
		common.PtrOrNil(s.group),
	)
}

func (s *URLTest) Now() string {
	if s.group.selectedOutboundTCP != nil {
		return s.group.selectedOutboundTCP.Tag()
	} else if s.group.selectedOutboundUDP != nil {
		return s.group.selectedOutboundUDP.Tag()
	}
	return ""
}

func (s *URLTest) All() []string {
	return s.tags
}

func (s *URLTest) References() []string {
	group := s.group
	if group == nil {
		return nil
	}
	var references []string
	if group.selectedOutboundTCP != nil {
		references = append(references, group.selectedOutboundTCP.Tag())
	}
	if group.selectedOutboundUDP != nil && group.selectedOutboundUDP != group.selectedOutboundTCP {
		references = append(references, group.selectedOutboundUDP.Tag())
	}
	return references
}

func (s *URLTest) URLTest(ctx context.Context) (map[string]uint16, error) {
	return s.group.URLTest(ctx)
}

func (s *URLTest) CheckOutbounds() {
	s.group.CheckOutbounds(s.ctx, true)
}

func (s *URLTest) PerformUpdateCheck() {
	s.group.performUpdateCheck()
}

func (s *URLTest) InterfaceUpdated(ctx context.Context) {
	group := s.group
	if group == nil {
		return
	}
	if group.pause.IsDevicePaused() || group.pause.IsNetworkPaused() {
		return
	}
	// Throughput observed on the previous network says nothing about this one, and
	// re-probing every outbound on every interface change would be expensive on
	// exactly the devices where interface changes are frequent. Drop the samples and
	// let selection fall back to latency until the next scheduled bandwidth sweep.
	group.resetBandwidth()
	go func() {
		s.checkAccess.Lock()
		defer s.checkAccess.Unlock()
		if ctx.Err() != nil {
			return
		}
		group.CheckOutbounds(ctx, true)
	}()
}

func (s *URLTest) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	s.group.Touch()
	var outbound adapter.Outbound
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		outbound = s.group.selectedOutboundTCP
	case N.NetworkUDP:
		outbound = s.group.selectedOutboundUDP
	default:
		return nil, E.Extend(N.ErrUnknownNetwork, network)
	}
	if outbound == nil {
		outbound, _ = s.group.Select(network)
	}
	if outbound == nil {
		return nil, E.New("missing supported outbound")
	}
	conn, err := outbound.DialContext(ctx, network, destination)
	if err == nil {
		return s.group.interruptGroup.NewConn(conn, interrupt.IsExternalConnectionFromContext(ctx)), nil
	}
	s.logger.ErrorContext(ctx, err)
	s.group.history.DeleteURLTestHistory(outbound.Tag())
	return nil, err
}

func (s *URLTest) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	s.group.Touch()
	outbound := s.group.selectedOutboundUDP
	if outbound == nil {
		outbound, _ = s.group.Select(N.NetworkUDP)
	}
	if outbound == nil {
		return nil, E.New("missing supported outbound")
	}
	conn, err := outbound.ListenPacket(ctx, destination)
	if err == nil {
		return s.group.interruptGroup.NewPacketConn(conn, interrupt.IsExternalConnectionFromContext(ctx)), nil
	}
	s.logger.ErrorContext(ctx, err)
	s.group.history.DeleteURLTestHistory(outbound.Tag())
	return nil, err
}

func (s *URLTest) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	ctx = interrupt.ContextWithIsExternalConnection(ctx)
	s.connection.NewConnection(ctx, s, conn, metadata, onClose)
}

func (s *URLTest) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	ctx = interrupt.ContextWithIsExternalConnection(ctx)
	s.connection.NewPacketConnection(ctx, s, conn, metadata, onClose)
}

type URLTestGroup struct {
	ctx                          context.Context
	outbound                     adapter.OutboundManager
	pause                        pause.Manager
	pauseCallback                *list.Element[pause.Callback]
	logger                       log.Logger
	outbounds                    []adapter.Outbound
	link                         string
	interval                     time.Duration
	tolerance                    uint16
	idleTimeout                  time.Duration
	history                      *urltest.HistoryStorage
	checking                     atomic.Bool
	selectedOutboundTCP          adapter.Outbound
	selectedOutboundUDP          adapter.Outbound
	interruptGroup               *interrupt.Group
	interruptExternalConnections bool
	access                       sync.Mutex
	updateAccess                 sync.Mutex
	ticker                       *time.Ticker
	close                        chan struct{}
	started                      bool
	lastActive                   common.TypedValue[time.Time]

	// bandwidth is nil unless bandwidth testing is enabled, which keeps every code
	// path below a no-op by default.
	bandwidth              *bandwidthTestOptions
	bandwidthChecking      atomic.Bool
	bandwidthTicker        *time.Ticker
	bandwidthPauseCallback *list.Element[pause.Callback]
	bandwidthAccess        sync.Mutex
	bandwidthHistory       map[string]*bandwidthState
	// bandwidthEpoch counts resetBandwidth calls. Workers capture it before probing
	// so a sample measured on a previous network path is dropped instead of written
	// back into the freshly cleared history (guarded by bandwidthAccess).
	bandwidthEpoch uint64
}

// bandwidthState holds the smoothing window for one outbound. Throughput samples are
// far noisier than latency samples — a probe landing during a transient burst can
// swing severalfold — so selection reads the smoothed value, never the last sample.
type bandwidthState struct {
	samples  []uint32
	smoothed uint32
	lastTest time.Time
}

type bandwidthTestOptions struct {
	link                string
	maxBytes            uint32
	timeout             time.Duration
	interval            time.Duration
	concurrency         int
	strategy            string
	latencyFloor        uint16 // milliseconds; zero disables the floor
	throughputTolerance uint16 // percent
	samples             int
}

const (
	defaultBandwidthMaxBytes = 256 * 1024
	// maxBandwidthMaxBytes caps what a single probe may pull. This is a ranking
	// signal, and the cost is paid on every outbound on every interval, potentially
	// over metered data.
	maxBandwidthMaxBytes = 1024 * 1024
	// defaultBandwidthConcurrency is deliberately far below the latency sweep's
	// fixed 10: these probes consume bandwidth, so running many at once makes them
	// contend with each other and skews every result.
	defaultBandwidthConcurrency = 2
	// defaultBandwidthIntervalMultiplier derives the bandwidth interval from the
	// latency interval, since the right cadence for a throughput probe is much lower
	// than for a liveness probe.
	defaultBandwidthIntervalMultiplier = 5
	defaultBandwidthTolerance          = 25
	defaultBandwidthSamples            = 3
)

func newBandwidthTestOptions(options *option.URLTestBandwidthTestOptions, interval time.Duration) (*bandwidthTestOptions, error) {
	if options == nil || !options.Enabled {
		return nil, nil
	}
	parsed := &bandwidthTestOptions{
		link:                options.URL,
		maxBytes:            options.MaxBytes,
		timeout:             time.Duration(options.Timeout),
		interval:            time.Duration(options.Interval),
		concurrency:         options.Concurrency,
		strategy:            options.Strategy,
		throughputTolerance: options.ThroughputTolerance,
		samples:             options.Samples,
	}
	if parsed.maxBytes == 0 {
		parsed.maxBytes = defaultBandwidthMaxBytes
	} else if parsed.maxBytes > maxBandwidthMaxBytes {
		return nil, E.New("bandwidth_test.max_bytes must be less or equal than ", maxBandwidthMaxBytes)
	}
	if parsed.link == "" {
		// Derived after max_bytes is resolved so the endpoint returns exactly the
		// number of bytes the probe intends to read.
		parsed.link = C.DefaultURLTestBandwidthURLPrefix + strconv.FormatUint(uint64(parsed.maxBytes), 10)
	}
	if parsed.timeout == 0 {
		parsed.timeout = C.DefaultURLTestBandwidthTimeout
	} else if parsed.timeout < 0 {
		return nil, E.New("bandwidth_test.timeout must be positive")
	}
	if parsed.interval == 0 {
		parsed.interval = interval * defaultBandwidthIntervalMultiplier
	} else if parsed.interval < 0 {
		return nil, E.New("bandwidth_test.interval must be positive")
	}
	if parsed.concurrency == 0 {
		parsed.concurrency = defaultBandwidthConcurrency
	} else if parsed.concurrency < 0 {
		return nil, E.New("bandwidth_test.concurrency must be positive")
	}
	switch parsed.strategy {
	case "":
		parsed.strategy = C.URLTestStrategyLatency
	case C.URLTestStrategyLatency, C.URLTestStrategyThroughput, C.URLTestStrategyThroughputWithLatencyFloor:
	default:
		return nil, E.New("unknown bandwidth_test.strategy: ", parsed.strategy)
	}
	latencyFloor := time.Duration(options.LatencyFloor)
	if latencyFloor < 0 {
		return nil, E.New("bandwidth_test.latency_floor must be positive")
	}
	if floorMilliseconds := latencyFloor.Milliseconds(); floorMilliseconds > math.MaxUint16 {
		// Delay is measured in uint16 milliseconds, so a larger floor can never
		// exclude anything anyway.
		parsed.latencyFloor = math.MaxUint16
	} else {
		parsed.latencyFloor = uint16(floorMilliseconds)
	}
	if parsed.throughputTolerance == 0 {
		parsed.throughputTolerance = defaultBandwidthTolerance
	}
	if parsed.samples == 0 {
		parsed.samples = defaultBandwidthSamples
	} else if parsed.samples < 0 {
		return nil, E.New("bandwidth_test.samples must be positive")
	}
	return parsed, nil
}

func NewURLTestGroup(ctx context.Context, outboundManager adapter.OutboundManager, logger log.Logger, outbounds []adapter.Outbound, link string, interval time.Duration, tolerance uint16, idleTimeout time.Duration, interruptExternalConnections bool, bandwidthOptions *option.URLTestBandwidthTestOptions) (*URLTestGroup, error) {
	if interval == 0 {
		interval = C.DefaultURLTestInterval
	}
	if tolerance == 0 {
		tolerance = 50
	}
	if idleTimeout == 0 {
		idleTimeout = C.DefaultURLTestIdleTimeout
	}
	if interval > idleTimeout {
		return nil, E.New("interval must be less or equal than idle_timeout")
	}
	bandwidth, err := newBandwidthTestOptions(bandwidthOptions, interval)
	if err != nil {
		return nil, err
	}
	if bandwidth != nil && bandwidth.interval > idleTimeout {
		logger.Warn("urltest: bandwidth_test.interval (", bandwidth.interval, ") is greater than idle_timeout (", idleTimeout, "); bandwidth probing will rarely run")
	}
	history := service.PtrFromContext[urltest.HistoryStorage](ctx)
	if history == nil {
		return nil, E.New("missing URL test history storage")
	}
	return &URLTestGroup{
		ctx:                          ctx,
		outbound:                     outboundManager,
		logger:                       logger,
		outbounds:                    outbounds,
		link:                         link,
		interval:                     interval,
		tolerance:                    tolerance,
		idleTimeout:                  idleTimeout,
		history:                      history,
		close:                        make(chan struct{}),
		pause:                        service.FromContext[pause.Manager](ctx),
		interruptGroup:               interrupt.NewGroup(),
		interruptExternalConnections: interruptExternalConnections,
		bandwidth:                    bandwidth,
		bandwidthHistory:             make(map[string]*bandwidthState),
	}, nil
}

func (g *URLTestGroup) PostStart() {
	g.access.Lock()
	defer g.access.Unlock()
	g.started = true
	g.lastActive.Store(time.Now())
	go g.CheckOutbounds(g.ctx, false)
}

func (g *URLTestGroup) Touch() {
	if !g.started {
		return
	}
	g.access.Lock()
	defer g.access.Unlock()
	if g.ticker != nil {
		g.lastActive.Store(time.Now())
		// The two tickers suspend independently, and the latency one suspends first
		// because it ticks more often, so the bandwidth ticker may still need
		// restarting even when the latency one is already running.
		g.startBandwidthTicker()
		return
	}
	ticker := time.NewTicker(g.interval)
	g.ticker = ticker
	g.pauseCallback = pause.RegisterTicker(g.pause, ticker, g.interval, nil)
	go g.loopCheck(ticker, g.close)
	g.startBandwidthTicker()
}

// startBandwidthTicker must be called with access held.
func (g *URLTestGroup) startBandwidthTicker() {
	if g.bandwidth == nil || g.bandwidthTicker != nil {
		return
	}
	// A separate ticker rather than a divisor of the latency one, so the two cadences
	// stay independent. It inherits the same pause registration and the same idle
	// suspension, so no probing happens while the group is unused or the device is
	// asleep.
	ticker := time.NewTicker(g.bandwidth.interval)
	g.bandwidthTicker = ticker
	g.bandwidthPauseCallback = pause.RegisterTicker(g.pause, ticker, g.bandwidth.interval, nil)
	go g.loopBandwidthCheck(ticker, g.close)
}

func (g *URLTestGroup) Close() error {
	g.access.Lock()
	defer g.access.Unlock()
	// Checked independently: either ticker may already have suspended itself on idle
	// timeout while the other is still running.
	if g.ticker == nil && g.bandwidthTicker == nil {
		return nil
	}
	if g.ticker != nil {
		g.ticker.Stop()
		g.ticker = nil
		g.pause.UnregisterCallback(g.pauseCallback)
		g.pauseCallback = nil
	}
	if g.bandwidthTicker != nil {
		g.bandwidthTicker.Stop()
		g.bandwidthTicker = nil
		g.pause.UnregisterCallback(g.bandwidthPauseCallback)
		g.bandwidthPauseCallback = nil
	}
	close(g.close)
	return nil
}

func (g *URLTestGroup) Select(network string) (adapter.Outbound, bool) {
	if g.bandwidth != nil && g.bandwidth.strategy != C.URLTestStrategyLatency {
		// Falls through to latency ranking until throughput samples exist, which
		// covers startup, the interval before the first bandwidth sweep, and the
		// case where every outbound was excluded by the latency floor.
		if outbound, exists, ranked := g.selectByThroughput(network); ranked {
			return outbound, exists
		}
	}
	return g.selectByLatency(network)
}

func (g *URLTestGroup) selectByLatency(network string) (adapter.Outbound, bool) {
	var minDelay uint16
	var minOutbound adapter.Outbound
	switch network {
	case N.NetworkTCP:
		if g.selectedOutboundTCP != nil {
			if history := g.history.LoadURLTestHistory(RealTag(g.outbound, g.selectedOutboundTCP)); history != nil {
				minOutbound = g.selectedOutboundTCP
				minDelay = history.Delay
			}
		}
	case N.NetworkUDP:
		if g.selectedOutboundUDP != nil {
			if history := g.history.LoadURLTestHistory(RealTag(g.outbound, g.selectedOutboundUDP)); history != nil {
				minOutbound = g.selectedOutboundUDP
				minDelay = history.Delay
			}
		}
	}
	for _, detour := range g.outbounds {
		if !common.Contains(detour.Network(), network) {
			continue
		}
		history := g.history.LoadURLTestHistory(RealTag(g.outbound, detour))
		if history == nil {
			continue
		}
		if minDelay == 0 || minDelay > history.Delay+g.tolerance {
			minDelay = history.Delay
			minOutbound = detour
		}
	}
	if minOutbound == nil {
		for _, detour := range g.outbounds {
			if !common.Contains(detour.Network(), network) {
				continue
			}
			return detour, false
		}
		return nil, false
	}
	return minOutbound, true
}

// selectByThroughput ranks eligible outbounds by smoothed throughput. The third
// return value reports whether any outbound was eligible at all; when it is false the
// caller falls back to latency ranking rather than leaving the group unselected.
func (g *URLTestGroup) selectByThroughput(network string) (adapter.Outbound, bool, bool) {
	var maxThroughput uint32
	var maxOutbound adapter.Outbound
	// Seed with the incumbent so it keeps the same advantage it has under latency
	// ranking: a challenger has to beat it by the tolerance, not merely tie it.
	var selected adapter.Outbound
	switch network {
	case N.NetworkTCP:
		selected = g.selectedOutboundTCP
	case N.NetworkUDP:
		selected = g.selectedOutboundUDP
	}
	if selected != nil {
		if throughput, eligible := g.candidateThroughput(selected); eligible {
			maxOutbound = selected
			maxThroughput = throughput
		}
	}
	for _, detour := range g.outbounds {
		if !common.Contains(detour.Network(), network) {
			continue
		}
		throughput, eligible := g.candidateThroughput(detour)
		if !eligible {
			continue
		}
		if maxOutbound == nil || beatsIncumbent(throughput, maxThroughput, g.bandwidth.throughputTolerance) {
			maxThroughput = throughput
			maxOutbound = detour
		}
	}
	if maxOutbound == nil {
		return nil, false, false
	}
	return maxOutbound, true, true
}

// candidateThroughput reports the smoothed throughput of detour and whether it may be
// ranked by it: the outbound needs a latency sample (liveness), a non-zero throughput
// sample, and — under throughput_with_latency_floor — a latency within the floor.
func (g *URLTestGroup) candidateThroughput(detour adapter.Outbound) (uint32, bool) {
	realTag := RealTag(g.outbound, detour)
	history := g.history.LoadURLTestHistory(realTag)
	if history == nil {
		return 0, false
	}
	if g.bandwidth.strategy == C.URLTestStrategyThroughputWithLatencyFloor &&
		g.bandwidth.latencyFloor > 0 &&
		history.Delay > g.bandwidth.latencyFloor {
		return 0, false
	}
	throughput := g.loadBandwidth(realTag)
	if throughput == 0 {
		return 0, false
	}
	return throughput, true
}

func (g *URLTestGroup) loadBandwidth(tag string) uint32 {
	g.bandwidthAccess.Lock()
	defer g.bandwidthAccess.Unlock()
	if state := g.bandwidthHistory[tag]; state != nil {
		return state.smoothed
	}
	return 0
}

// currentBandwidthEpoch returns the sample generation counter. A worker captures it
// before probing and hands it to recordBandwidthChecked, which drops the sample if the
// generation has since been reset.
func (g *URLTestGroup) currentBandwidthEpoch() uint64 {
	g.bandwidthAccess.Lock()
	defer g.bandwidthAccess.Unlock()
	return g.bandwidthEpoch
}

// recordBandwidthChecked appends a sample to the smoothing window and returns the new
// smoothed value, unless the probe started before the last resetBandwidth — a sample
// measured on the previous network says nothing about the current one. A failed probe
// is recorded as a zero sample rather than dropped, so that a genuinely broken path
// decays out of contention while a single transient failure is absorbed by the median.
func (g *URLTestGroup) recordBandwidthChecked(epoch uint64, tag string, throughput uint32) (uint32, bool) {
	g.bandwidthAccess.Lock()
	defer g.bandwidthAccess.Unlock()
	if epoch != g.bandwidthEpoch {
		return 0, false
	}
	state := g.bandwidthHistory[tag]
	if state == nil {
		state = new(bandwidthState)
		g.bandwidthHistory[tag] = state
	}
	state.samples = append(state.samples, throughput)
	if len(state.samples) > g.bandwidth.samples {
		state.samples = state.samples[len(state.samples)-g.bandwidth.samples:]
	}
	state.lastTest = time.Now()
	state.smoothed = medianThroughput(state.samples)
	return state.smoothed, true
}

func (g *URLTestGroup) bandwidthExpired(tag string) bool {
	g.bandwidthAccess.Lock()
	defer g.bandwidthAccess.Unlock()
	state := g.bandwidthHistory[tag]
	return state == nil || time.Since(state.lastTest) >= g.bandwidth.interval
}

// resetBandwidth discards every sample, so selection falls back to latency until the
// next bandwidth sweep re-measures the new path. Bumping the epoch also invalidates
// in-flight probes, whose samples belong to the previous path.
func (g *URLTestGroup) resetBandwidth() {
	if g.bandwidth == nil {
		return
	}
	g.bandwidthAccess.Lock()
	clear(g.bandwidthHistory)
	g.bandwidthEpoch++
	g.bandwidthAccess.Unlock()
}

// beatsIncumbent reports whether a challenger's throughput clears the incumbent's by
// the configured margin. The hysteresis is relative rather than absolute because
// throughput ratios are the meaningful comparison: a fixed byte-rate band would mean
// something entirely different on a slow link than on a fast one.
func beatsIncumbent(challenger uint32, incumbent uint32, tolerancePercent uint16) bool {
	return uint64(challenger) > uint64(incumbent)*uint64(100+tolerancePercent)/100
}

func medianThroughput(samples []uint32) uint32 {
	if len(samples) == 0 {
		return 0
	}
	sorted := slices.Clone(samples)
	slices.Sort(sorted)
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle]
	}
	return uint32((uint64(sorted[middle-1]) + uint64(sorted[middle])) / 2)
}

func (g *URLTestGroup) loopBandwidthCheck(ticker *time.Ticker, closeChan <-chan struct{}) {
	g.bandwidthTest(g.ctx, false)
	for {
		select {
		case <-closeChan:
			return
		case <-ticker.C:
		}
		if time.Since(g.lastActive.Load()) > g.idleTimeout {
			g.access.Lock()
			if g.bandwidthTicker == ticker {
				g.bandwidthTicker.Stop()
				g.bandwidthTicker = nil
				g.pause.UnregisterCallback(g.bandwidthPauseCallback)
				g.bandwidthPauseCallback = nil
			}
			g.access.Unlock()
			return
		}
		g.bandwidthTest(g.ctx, false)
	}
}

func (g *URLTestGroup) bandwidthTest(ctx context.Context, force bool) {
	if g.bandwidth == nil {
		return
	}
	if g.bandwidthChecking.Swap(true) {
		return
	}
	defer g.bandwidthChecking.Store(false)
	b, _ := batch.New(ctx, batch.WithConcurrencyNum[any](g.bandwidth.concurrency))
	checked := make(map[string]bool)
	var workers int
	for _, detour := range g.outbounds {
		tag := detour.Tag()
		realTag := RealTag(g.outbound, detour)
		if checked[realTag] {
			continue
		}
		if !force && !g.bandwidthExpired(realTag) {
			continue
		}
		checked[realTag] = true
		p, loaded := g.outbound.Outbound(realTag)
		if !loaded {
			continue
		}
		workers++
		b.Go(realTag, func() (any, error) {
			epoch := g.currentBandwidthEpoch()
			testCtx, cancel := context.WithTimeout(g.ctx, g.bandwidth.timeout)
			defer cancel()
			var throughput, readBytes uint32
			result, err := urltest.BandwidthTest(testCtx, g.bandwidth.link, p, g.bandwidth.maxBytes, g.bandwidth.timeout)
			if err != nil {
				// Only the throughput sample is affected; the latency history is left
				// alone, so a failed bandwidth probe cannot deselect an outbound that
				// the latency probe still considers reachable.
				g.logger.Debug("outbound ", tag, " bandwidth test failed: ", err)
			} else {
				throughput = result.Throughput()
				readBytes = result.Bytes
				g.logger.Debug("outbound ", tag, " bandwidth: ", throughput, " B/s over ", readBytes, " bytes")
			}
			if smoothed, recorded := g.recordBandwidthChecked(epoch, realTag, throughput); recorded {
				g.history.StoreURLTestBandwidth(realTag, smoothed, readBytes)
			}
			return nil, nil
		})
	}
	// Every worker carries its own deadline, but #4255 shows a dialer can park in a
	// read that ignores both the context deadline and the client timeout. batch.Wait
	// has no deadline of its own, so one such worker would leave bandwidthChecking
	// set forever and silently disable every later sweep. Bound the round by the time
	// the queued workers legitimately need at worst — ceil(workers/concurrency)
	// timeout-sized waves, plus one wave of slack — and abandon it when even that
	// expires. The stuck worker's goroutine outlives the round either way; giving up
	// on it costs one leaked goroutine instead of the whole feature.
	waves := (workers + g.bandwidth.concurrency - 1) / g.bandwidth.concurrency
	budget := time.Duration(waves+1) * g.bandwidth.timeout
	waitDone := make(chan struct{})
	go func() {
		b.Wait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
	case <-time.After(budget):
		g.logger.Warn("urltest: bandwidth round exceeded ", budget, "; a probe is ignoring its deadline, abandoning this round")
	}
	g.performUpdateCheck()
}

func (g *URLTestGroup) loopCheck(ticker *time.Ticker, closeChan <-chan struct{}) {
	if time.Since(g.lastActive.Load()) > g.interval {
		g.lastActive.Store(time.Now())
		g.CheckOutbounds(g.ctx, false)
	}
	for {
		select {
		case <-closeChan:
			return
		case <-ticker.C:
		}
		if time.Since(g.lastActive.Load()) > g.idleTimeout {
			g.access.Lock()
			if g.ticker == ticker {
				g.ticker.Stop()
				g.ticker = nil
				g.pause.UnregisterCallback(g.pauseCallback)
				g.pauseCallback = nil
			}
			g.access.Unlock()
			return
		}
		g.CheckOutbounds(g.ctx, false)
	}
}

func (g *URLTestGroup) CheckOutbounds(ctx context.Context, force bool) {
	_, _ = g.urlTest(ctx, force)
}

func (g *URLTestGroup) URLTest(ctx context.Context) (map[string]uint16, error) {
	return g.urlTest(ctx, true)
}

func (g *URLTestGroup) urlTest(ctx context.Context, force bool) (map[string]uint16, error) {
	if g.checking.Swap(true) {
		return make(map[string]uint16), nil
	}
	defer g.checking.Store(false)
	result := URLTestOutbounds(ctx, g.outbound, g.history, g.logger, g.outbounds, g.link, g.interval, force)
	g.performUpdateCheck()
	return result, nil
}

type urlTestResult struct {
	delay uint16
	err   error
}

type urlTestBatch struct {
	ctx      context.Context
	outbound adapter.OutboundManager
	history  *urltest.HistoryStorage
	logger   log.Logger
	batch    *batch.Batch[any]
	checked  map[string]bool
	groups   []adapter.OutboundGroup
	access   sync.Mutex
	result   map[string]uint16
}

func URLTestOutbounds(ctx context.Context, outboundManager adapter.OutboundManager, history *urltest.HistoryStorage, logger log.Logger, outbounds []adapter.Outbound, link string, interval time.Duration, force bool) map[string]uint16 {
	b, _ := batch.New(ctx, batch.WithConcurrencyNum[any](10))
	testBatch := &urlTestBatch{
		ctx:      ctx,
		outbound: outboundManager,
		history:  history,
		logger:   logger,
		batch:    b,
		checked:  make(map[string]bool),
		result:   make(map[string]uint16),
	}
	testBatch.test(outbounds, link, interval, force)
	b.Wait()
	for _, outboundGroup := range testBatch.groups {
		groupHistory := history.LoadURLTestHistory(RealTag(outboundManager, outboundGroup))
		if groupHistory != nil {
			testBatch.result[outboundGroup.Tag()] = groupHistory.Delay
		}
	}
	return testBatch.result
}

func (b *urlTestBatch) test(outbounds []adapter.Outbound, link string, interval time.Duration, force bool) {
	for _, detour := range outbounds {
		tag := detour.Tag()
		if b.checked[tag] {
			continue
		}
		switch nested := detour.(type) {
		case *URLTest:
			b.checked[tag] = true
			b.groups = append(b.groups, nested)
			b.batch.Go(tag, func() (any, error) {
				nestedResult, _ := nested.group.urlTest(b.ctx, force)
				b.access.Lock()
				maps.Copy(b.result, nestedResult)
				b.access.Unlock()
				return nil, nil
			})
		case adapter.OutboundGroup:
			b.checked[tag] = true
			b.groups = append(b.groups, nested)
			b.test(common.FilterNotNil(common.Map(nested.All(), func(it string) adapter.Outbound {
				member, _ := b.outbound.Outbound(it)
				return member
			})), link, interval, force)
		default:
			history := b.history.LoadURLTestHistory(tag)
			if !force && history != nil && time.Since(history.Time) < interval {
				continue
			}
			b.checked[tag] = true
			b.batch.Go(tag, func() (any, error) {
				testCtx, cancel := context.WithTimeout(b.ctx, C.TCPTimeout)
				defer cancel()
				testChan := make(chan urlTestResult, 1)
				go func() {
					delay, testErr := urltest.URLTest(testCtx, link, detour)
					testChan <- urlTestResult{delay, testErr}
				}()
				var testResult urlTestResult
				select {
				case testResult = <-testChan:
				case <-testCtx.Done():
					testResult.err = testCtx.Err()
				}
				if testResult.err != nil {
					b.logger.Debug("outbound ", tag, " unavailable: ", testResult.err)
					b.history.DeleteURLTestHistory(tag)
				} else {
					b.logger.Debug("outbound ", tag, " available: ", testResult.delay, "ms")
					b.history.StoreURLTestHistory(tag, &adapter.URLTestHistory{
						Time:  time.Now(),
						Delay: testResult.delay,
					})
					b.access.Lock()
					b.result[tag] = testResult.delay
					b.access.Unlock()
				}
				return nil, nil
			})
		}
	}
}

func (g *URLTestGroup) performUpdateCheck() {
	// The latency sweep and the bandwidth sweep have independent re-entrancy guards
	// and can finish at the same time, so this needs its own lock against two
	// concurrent writers to the selected outbounds.
	g.updateAccess.Lock()
	defer g.updateAccess.Unlock()
	var (
		updated  bool
		selected bool
	)
	if outbound, exists := g.Select(N.NetworkTCP); outbound != nil && (g.selectedOutboundTCP == nil || (exists && outbound != g.selectedOutboundTCP)) {
		if g.selectedOutboundTCP != nil {
			updated = true
		}
		g.selectedOutboundTCP = outbound
		selected = true
	}
	if outbound, exists := g.Select(N.NetworkUDP); outbound != nil && (g.selectedOutboundUDP == nil || (exists && outbound != g.selectedOutboundUDP)) {
		if g.selectedOutboundUDP != nil {
			updated = true
		}
		g.selectedOutboundUDP = outbound
		selected = true
	}
	if updated {
		g.interruptGroup.Interrupt(g.interruptExternalConnections)
	}
	if selected {
		g.history.NotifyUpdated()
	}
}
