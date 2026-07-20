package group

import (
	"context"
	"hash/crc32"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/urltest"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/batch"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/pause"
)

func RegisterLoadBalance(registry *outbound.Registry) {
	outbound.Register[option.LoadBalanceOutboundOptions](registry, C.TypeLoadBalance, NewLoadBalance)
}

var (
	_ adapter.OutboundGroup           = (*LoadBalance)(nil)
	_ adapter.ConnectionHandler       = (*LoadBalance)(nil)
	_ adapter.PacketConnectionHandler = (*LoadBalance)(nil)
	_ adapter.InterfaceUpdateListener = (*LoadBalance)(nil)
)

type LoadBalance struct {
	outbound.Adapter
	ctx         context.Context
	outbound    adapter.OutboundManager
	connection  adapter.ConnectionManager
	logger      logger.ContextLogger
	tags        []string
	link        string
	strategy    string
	balancer    balancer
	outbounds   []adapter.Outbound
	interval    time.Duration
	idleTimeout time.Duration
	group       *LoadBalanceTestGroup
}

func NewLoadBalance(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.LoadBalanceOutboundOptions) (adapter.Outbound, error) {
	outbound := &LoadBalance{
		Adapter:     outbound.NewAdapter(C.TypeLoadBalance, tag, []string{N.NetworkTCP, N.NetworkUDP}, options.Outbounds),
		ctx:         ctx,
		outbound:    service.FromContext[adapter.OutboundManager](ctx),
		connection:  service.FromContext[adapter.ConnectionManager](ctx),
		logger:      logger,
		tags:        options.Outbounds,
		link:        options.URL,
		interval:    time.Duration(options.Interval),
		idleTimeout: time.Duration(options.IdleTimeout),
		strategy:    options.Strategy,
	}
	if len(outbound.tags) == 0 {
		return nil, E.New("missing tags")
	}
	return outbound, nil
}

func (s *LoadBalance) Start() error {
	s.outbounds = make([]adapter.Outbound, 0, len(s.tags))
	for i, tag := range s.tags {
		detour, loaded := s.outbound.Outbound(tag)
		if !loaded {
			return E.New("outbound ", i, " not found: ", tag)
		}
		s.outbounds = append(s.outbounds, detour)
	}
	switch s.strategy {
	case "", "round-robin":
		s.balancer = &roundRobinBalancer{
			outbounds: s.outbounds,
		}
	case "least-connections":
		balancer := &leastConnectionsBalancer{
			outbounds: s.outbounds,
			counts:    make([]atomic.Int64, len(s.outbounds)),
			tags:      make([]string, len(s.outbounds)),
		}
		for i, c := range s.outbounds {
			balancer.tags[i] = c.Tag()
		}
		s.balancer = balancer
	case "source-hash":
		s.balancer = &sourceHashBalancer{
			outbounds: s.outbounds,
		}
	case "consistent-hash":
		s.balancer = &consistentHashBalancer{
			outbounds: s.outbounds,
		}
	default:
		return E.New("unsupported load balance strategy: ", s.strategy)
	}
	group, err := NewLoadBalanceTestGroup(s.ctx, s.outbound, s.logger, s.outbounds, s.link, s.interval, s.idleTimeout)
	if err != nil {
		return err
	}
	s.group = group
	return nil
}

func (s *LoadBalance) PostStart() error {
	s.group.PostStart()
	return nil
}

func (s *LoadBalance) Close() error {
	return common.Close(
		common.PtrOrNil(s.group),
	)
}

func (s *LoadBalance) Now() string {
	return ""
}

func (s *LoadBalance) All() []string {
	return s.tags
}

func (s *LoadBalance) URLTest(ctx context.Context) (map[string]uint16, error) {
	return s.group.URLTest(ctx)
}

func (s *LoadBalance) CheckOutbounds(force bool) {
	s.group.CheckOutbounds(force)
}

func (s *LoadBalance) InterfaceUpdated() {
	group := s.group
	if group == nil {
		return
	}
	if group.pause.IsDevicePaused() || group.pause.IsNetworkPaused() {
		return
	}
	go group.CheckOutbounds(true)
}

func (s *LoadBalance) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	s.group.Touch()
	var outbound adapter.Outbound
	metadata := adapter.ContextFrom(ctx)
	outbound = s.balancer.Select(metadata)
	conn, err := outbound.DialContext(ctx, network, destination)
	if err == nil {
		return conn, nil
	}
	s.logger.ErrorContext(ctx, err)
	return nil, err
}

func (s *LoadBalance) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	s.group.Touch()
	var outbound adapter.Outbound
	metadata := adapter.ContextFrom(ctx)
	outbound = s.balancer.Select(metadata)
	conn, err := outbound.ListenPacket(ctx, destination)
	if err == nil {
		return conn, nil
	}
	s.logger.ErrorContext(ctx, err)
	return nil, err
}

func (s *LoadBalance) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	outbound := s.balancer.Select(&metadata)
	s.balancer.OnOpen(outbound.Tag())
	onClose = s.wrapOnClose(outbound.Tag(), onClose)
	s.connection.NewConnection(ctx, s, conn, metadata, onClose)
}

func (s *LoadBalance) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	outbound := s.balancer.Select(&metadata)
	s.balancer.OnOpen(outbound.Tag())
	onClose = s.wrapOnClose(outbound.Tag(), onClose)
	s.connection.NewPacketConnection(ctx, s, conn, metadata, onClose)
}

func (s *LoadBalance) wrapOnClose(tag string, onClose N.CloseHandlerFunc) N.CloseHandlerFunc {
	return N.CloseHandlerFunc(func(err error) {
		s.balancer.OnClose(tag)
		if onClose != nil {
			onClose(err)
		}
	})
}

type LoadBalanceTestGroup struct {
	ctx           context.Context
	outbound      adapter.OutboundManager
	pause         pause.Manager
	pauseCallback *list.Element[pause.Callback]
	logger        log.Logger
	outbounds     []adapter.Outbound
	link          string
	interval      time.Duration
	idleTimeout   time.Duration
	history       *urltest.HistoryStorage
	checking      atomic.Bool
	access        sync.Mutex
	ticker        *time.Ticker
	close         chan struct{}
	started       bool
	lastActive    common.TypedValue[time.Time]
}

func NewLoadBalanceTestGroup(ctx context.Context, outboundManager adapter.OutboundManager, logger log.Logger, outbounds []adapter.Outbound, link string, interval time.Duration, idleTimeout time.Duration) (*LoadBalanceTestGroup, error) {
	if interval == 0 {
		interval = C.DefaultURLTestInterval
	}
	if idleTimeout == 0 {
		idleTimeout = C.DefaultURLTestIdleTimeout
	}
	if interval > idleTimeout {
		return nil, E.New("interval must be less or equal than idle_timeout")
	}
	history := service.PtrFromContext[urltest.HistoryStorage](ctx)
	if history == nil {
		return nil, E.New("missing URL test history storage")
	}
	return &LoadBalanceTestGroup{
		ctx:         ctx,
		outbound:    outboundManager,
		logger:      logger,
		outbounds:   outbounds,
		link:        link,
		interval:    interval,
		idleTimeout: idleTimeout,
		history:     history,
		close:       make(chan struct{}),
		pause:       service.FromContext[pause.Manager](ctx),
	}, nil
}

func (g *LoadBalanceTestGroup) PostStart() {
	g.access.Lock()
	defer g.access.Unlock()
	g.started = true
	g.lastActive.Store(time.Now())
	go g.CheckOutbounds(false)
}

func (g *LoadBalanceTestGroup) Touch() {
	if !g.started {
		return
	}
	g.access.Lock()
	defer g.access.Unlock()
	if g.ticker != nil {
		g.lastActive.Store(time.Now())
		return
	}
	ticker := time.NewTicker(g.interval)
	g.ticker = ticker
	g.pauseCallback = pause.RegisterTicker(g.pause, ticker, g.interval, nil)
	go g.loopCheck(ticker, g.close)
}

func (g *LoadBalanceTestGroup) Close() error {
	g.access.Lock()
	defer g.access.Unlock()
	if g.ticker == nil {
		return nil
	}
	g.ticker.Stop()
	g.ticker = nil
	g.pause.UnregisterCallback(g.pauseCallback)
	g.pauseCallback = nil
	close(g.close)
	return nil
}

func (g *LoadBalanceTestGroup) loopCheck(ticker *time.Ticker, closeChan <-chan struct{}) {
	if time.Since(g.lastActive.Load()) > g.interval {
		g.lastActive.Store(time.Now())
		g.CheckOutbounds(false)
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
		g.CheckOutbounds(false)
	}
}

func (g *LoadBalanceTestGroup) CheckOutbounds(force bool) {
	_, _ = g.urlTest(g.ctx, force)
}

func (g *LoadBalanceTestGroup) URLTest(ctx context.Context) (map[string]uint16, error) {
	return g.urlTest(ctx, false)
}

func (g *LoadBalanceTestGroup) urlTest(ctx context.Context, force bool) (map[string]uint16, error) {
	if g.checking.Swap(true) {
		return nil, nil
	}
	defer g.checking.Store(false)
	b, _ := batch.New(ctx, batch.WithConcurrencyNum[any](10))
	checked := make(map[string]bool)
	for _, detour := range g.outbounds {
		tag := detour.Tag()
		realTag := RealTag(detour)
		if checked[realTag] {
			continue
		}
		history := g.history.LoadURLTestHistory(realTag)
		if !force && history != nil && time.Since(history.Time) < g.interval {
			continue
		}
		checked[realTag] = true
		p, loaded := g.outbound.Outbound(realTag)
		if !loaded {
			continue
		}
		b.Go(realTag, func() (any, error) {
			testCtx, cancel := context.WithTimeout(g.ctx, C.TCPTimeout)
			defer cancel()
			t, err := urltest.URLTest(testCtx, g.link, p)
			if err != nil {
				g.logger.Debug("outbound ", tag, " unavailable: ", err)
				g.history.DeleteURLTestHistory(realTag)
			} else {
				g.logger.Debug("outbound ", tag, " available: ", t, "ms")
				g.history.StoreURLTestHistory(realTag, &adapter.URLTestHistory{
					Time:  time.Now(),
					Delay: t,
				})
			}
			return nil, nil
		})
	}
	b.Wait()
	return nil, nil
}

type balancer interface {
	Select(ctx *adapter.InboundContext) adapter.Outbound
	OnOpen(tag string)
	OnClose(tag string)
}

type roundRobinBalancer struct {
	outbounds []adapter.Outbound
	next      atomic.Uint64
}

func (b *roundRobinBalancer) Select(_ *adapter.InboundContext) adapter.Outbound {
	idx := b.next.Add(1) - 1
	return b.outbounds[idx%uint64(len(b.outbounds))]
}

func (b *roundRobinBalancer) OnOpen(_ string)  {}
func (b *roundRobinBalancer) OnClose(_ string) {}

type leastConnectionsBalancer struct {
	outbounds []adapter.Outbound
	counts    []atomic.Int64
	tags      []string
}

func (b *leastConnectionsBalancer) Select(_ *adapter.InboundContext) adapter.Outbound {
	minIdx := 0
	minVal := b.counts[0].Load()
	for i := 1; i < len(b.counts); i++ {
		val := b.counts[i].Load()
		if val < minVal {
			minVal = val
			minIdx = i
		}
	}
	return b.outbounds[minIdx]
}

func (b *leastConnectionsBalancer) OnOpen(tag string) {
	for i, t := range b.tags {
		if t == tag {
			b.counts[i].Add(1)
			return
		}
	}
}

func (b *leastConnectionsBalancer) OnClose(tag string) {
	for i, t := range b.tags {
		if t == tag {
			b.counts[i].Add(-1)
			return
		}
	}
}

type sourceHashBalancer struct {
	outbounds []adapter.Outbound
}

func (b *sourceHashBalancer) Select(ctx *adapter.InboundContext) adapter.Outbound {
	source := sourceFromCtx(ctx)
	idx := hashToIndex(source, len(b.outbounds))
	return b.outbounds[idx]
}

func (b *sourceHashBalancer) OnOpen(_ string)  {}
func (b *sourceHashBalancer) OnClose(_ string) {}

type consistentHashBalancer struct {
	outbounds []adapter.Outbound
}

func (b *consistentHashBalancer) Select(ctx *adapter.InboundContext) adapter.Outbound {
	source := sourceFromCtx(ctx)
	if source == "" || len(b.outbounds) == 0 {
		return b.outbounds[0]
	}
	idx := jumpHash(uint64(crc32.ChecksumIEEE([]byte(source))), int32(len(b.outbounds)))
	return b.outbounds[idx]
}

func (b *consistentHashBalancer) OnOpen(_ string)  {}
func (b *consistentHashBalancer) OnClose(_ string) {}

func hashToIndex(source string, n int) int {
	if source == "" || n == 0 {
		return 0
	}
	return int(crc32.ChecksumIEEE([]byte(source)) % uint32(n))
}

func jumpHash(key uint64, buckets int32) int32 {
	var b, j int64
	for j < int64(buckets) {
		b = j
		key = key*2862933555777941757 + 1
		j = int64(float64(b+1) * (float64(int64(1)<<31) / float64((key>>33)+1)))
	}
	return int32(b)
}

func sourceFromCtx(ctx *adapter.InboundContext) string {
	if ctx == nil {
		return ""
	}
	return ctx.Source.Addr.String()
}
