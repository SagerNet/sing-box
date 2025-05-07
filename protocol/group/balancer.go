package group

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/urltest"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

func RegisterBalancer(registry *outbound.Registry) {
	outbound.Register[option.BalancerOutboundOptions](registry, C.TypeBalancer, NewBalancer)
}

type Balancer struct {
	outbound.Adapter
	ctx           context.Context
	router        adapter.Router
	outboundMgr   adapter.OutboundManager
	connMgr       adapter.ConnectionManager
	logger        log.ContextLogger
	tags          []string
	link          string
	interval      time.Duration
	historyTTL    time.Duration
	forceRandom   bool
	retryCount    int
	retryInterval time.Duration
	group         *BalancerGroup
}

func NewBalancer(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, opts option.BalancerOutboundOptions) (adapter.Outbound, error) {
	o := &Balancer{
		Adapter:       outbound.NewAdapter(C.TypeBalancer, tag, []string{N.NetworkTCP, N.NetworkUDP}, opts.Outbounds),
		ctx:           ctx,
		router:        router,
		outboundMgr:   service.FromContext[adapter.OutboundManager](ctx),
		connMgr:       service.FromContext[adapter.ConnectionManager](ctx),
		logger:        logger,
		tags:          opts.Outbounds,
		link:          opts.URL,
		interval:      time.Duration(opts.Interval),
		historyTTL:    time.Duration(opts.HistoryTTL),
		forceRandom:   opts.ForceRandom,
		retryCount:    opts.RetryCount,
		retryInterval: time.Duration(opts.RetryInterval),
	}
	if len(o.tags) == 0 {
		return nil, E.New("missing tags")
	}
	return o, nil
}

func (b *Balancer) Start() error {
	outs := make([]adapter.Outbound, 0, len(b.tags))
	for i, tag := range b.tags {
		d, ok := b.outboundMgr.Outbound(tag)
		if !ok {
			return E.New("outbound ", i, " not found: ", tag)
		}
		outs = append(outs, d)
	}
	g := NewBalancerGroup(b.ctx, b.outboundMgr, b.logger, outs, b.link, b.interval, b.historyTTL, b.forceRandom, b.retryCount, b.retryInterval)
	b.group = g
	return nil
}

func (b *Balancer) PostStart() error {
	b.group.PostStart()
	return nil
}

func (b *Balancer) Close() error {
	return common.Close(common.PtrOrNil(b.group))
}

func (b *Balancer) DialContext(ctx context.Context, network string, dest M.Socksaddr) (net.Conn, error) {
	o, err := b.group.SelectOutbound(dest, network)
	if err != nil {
		return nil, err
	}
	return o.DialContext(ctx, network, dest)
}

func (b *Balancer) ListenPacket(ctx context.Context, dest M.Socksaddr) (net.PacketConn, error) {
	o, err := b.group.SelectOutbound(dest, N.NetworkUDP)
	if err != nil {
		return nil, err
	}
	return o.ListenPacket(ctx, dest)
}

func (b *Balancer) NewConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	b.connMgr.NewConnection(ctx, b, conn, metadata, onClose)
}

func (b *Balancer) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	b.connMgr.NewPacketConnection(ctx, b, conn, metadata, onClose)
}

type BalancerGroup struct {
	ctx           context.Context
	outboundMgr   adapter.OutboundManager
	logger        log.Logger
	outbounds     []adapter.Outbound
	link          string
	interval      time.Duration
	historyTTL    time.Duration
	forceRandom   bool
	retryCount    int
	retryInterval time.Duration

	availLock    sync.RWMutex
	availability map[string]bool
	initialized  bool

	histLock sync.RWMutex
	history  map[string]historyEntry

	ticker *time.Ticker
	close  chan struct{}
}

type historyEntry struct {
	tag string
	t   time.Time
}

func NewBalancerGroup(ctx context.Context, om adapter.OutboundManager, logger log.Logger, outs []adapter.Outbound, link string, interval, ttl time.Duration, force bool, retryCount int, retryInterval time.Duration) *BalancerGroup {
	if link == "" {
		link = "https://www.gstatic.com/generate_204"
	}
	if interval == 0 {
		interval = C.DefaultURLTestInterval
	}
	if ttl == 0 {
		ttl = 5 * time.Minute
	}
	if retryCount <= 0 {
		retryCount = 3
	}
	if retryInterval == 0 {
		retryInterval = 1 * time.Second
	}

	availability := make(map[string]bool)
	for _, out := range outs {
		availability[out.Tag()] = true
	}

	return &BalancerGroup{
		ctx:           ctx,
		outboundMgr:   om,
		logger:        logger,
		outbounds:     outs,
		link:          link,
		interval:      interval,
		historyTTL:    ttl,
		forceRandom:   force,
		retryCount:    retryCount,
		retryInterval: retryInterval,
		availability:  availability,
		history:       make(map[string]historyEntry),
		close:         make(chan struct{}),
		initialized:   true,
	}
}

func (g *BalancerGroup) PostStart() {
	g.logger.Debug("starting balancer group with ", len(g.outbounds), " outbounds")

	g.ticker = time.NewTicker(g.interval)
	go g.loop()

	go g.doCheckAvailability()
}

func (g *BalancerGroup) Close() error {
	if g.ticker != nil {
		g.ticker.Stop()
	}
	close(g.close)
	return nil
}

func (g *BalancerGroup) loop() {
	for {
		select {
		case <-g.close:
			return
		case <-g.ticker.C:
			g.checkAvailability()
		}
	}
}

func (g *BalancerGroup) checkAvailability() {
	go g.doCheckAvailability()
}

func (g *BalancerGroup) doCheckAvailability() {
	g.logger.Debug("checking availability of ", len(g.outbounds), " outbounds")

	results := make(map[string]bool)
	statuses := make([]string, 0, len(g.outbounds))

	for _, d := range g.outbounds {
		tag := d.Tag()
		available := false
		var finalErr error
		var finalResult uint16

		testCtx, cancel := context.WithTimeout(g.ctx, C.TCPTimeout)
		result, err := urltest.URLTest(testCtx, g.link, d)
		cancel()

		if err == nil {
			available = true
			finalResult = result
		} else {
			g.logger.Debug("outbound ", tag, " test failed, retrying (1/", g.retryCount, "): ", err)
			finalErr = err

			for i := 0; i < g.retryCount; i++ {
				select {
				case <-time.After(g.retryInterval):
				case <-g.close:
					return
				}

				testCtx, cancel := context.WithTimeout(g.ctx, C.TCPTimeout)
				result, err := urltest.URLTest(testCtx, g.link, d)
				cancel()

				if err == nil {
					g.logger.Debug("outbound ", tag, " retry success on attempt ", i+1, "/", g.retryCount)
					available = true
					finalResult = result
					finalErr = nil
					break
				} else {
					finalErr = err
					g.logger.Debug("outbound ", tag, " retry failed (", i+1, "/", g.retryCount, "): ", err)
				}
			}
		}

		results[tag] = available

		if finalErr != nil {
			g.logger.Warn("outbound ", tag, " test to ", g.link, " unavailable after ", g.retryCount, " attempts: ", finalErr)
			statuses = append(statuses, fmt.Sprintf("%s:unavailable", tag))
		} else {
			g.logger.Debug("outbound ", tag, " test to ", g.link, " available in ", finalResult, "ms")
			statuses = append(statuses, fmt.Sprintf("%s:%dms", tag, finalResult))
		}
	}

	g.availLock.Lock()
	for tag, available := range results {
		g.availability[tag] = available
	}
	g.availLock.Unlock()

	g.logger.Debug("URLTest details: ", strings.Join(statuses, ", "))
}

func (g *BalancerGroup) SelectOutbound(dest M.Socksaddr, network string) (adapter.Outbound, error) {
	key := dest.String()

	g.histLock.RLock()
	he, ok := g.history[key]
	g.histLock.RUnlock()

	if !g.forceRandom && ok && time.Since(he.t) < g.historyTTL {
		g.availLock.RLock()
		avail := g.availability[he.tag]
		g.availLock.RUnlock()
		if avail {
			g.logger.Debug("reuse outbound ", he.tag, " for destination ", key)
			o, _ := g.outboundMgr.Outbound(he.tag)
			return o, nil
		}
	}

	candidates := make([]adapter.Outbound, 0)
	g.availLock.RLock()
	for _, d := range g.outbounds {
		if g.availability[d.Tag()] {
			candidates = append(candidates, d)
		}
	}
	g.availLock.RUnlock()

	if len(candidates) == 0 {
		statuses := make([]string, 0, len(g.outbounds))
		g.availLock.RLock()
		for _, d := range g.outbounds {
			tag := d.Tag()
			statuses = append(statuses, fmt.Sprintf("%s:%t", tag, g.availability[tag]))
		}
		g.availLock.RUnlock()
		g.logger.Warn("availability map: ", strings.Join(statuses, ", "))
		g.logger.Warn("no available outbound for destination ", key)
		return nil, E.New("no available outbound")
	} else {
		g.logger.Debug("available outbounds: ", len(candidates), " for destination ", key)
	}

	o := candidates[rand.Intn(len(candidates))]
	g.logger.Debug("selected outbound ", o.Tag(), " for destination ", key)

	g.histLock.Lock()
	g.history[key] = historyEntry{tag: o.Tag(), t: time.Now()}
	g.histLock.Unlock()

	return o, nil
}
