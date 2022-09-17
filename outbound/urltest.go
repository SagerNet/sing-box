package outbound

import (
	"context"
	"net"
	"sort"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/urltest"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/batch"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var (
	_ adapter.Outbound      = (*URLTest)(nil)
	_ adapter.OutboundGroup = (*URLTest)(nil)
)

type URLTest struct {
	myOutboundAdapter
	tags      []string
	link      string
	interval  time.Duration
	tolerance uint16
	group     *URLTestGroup
}

func NewURLTest(router adapter.Router, logger log.ContextLogger, tag string, options option.URLTestOutboundOptions) (*URLTest, error) {
	outbound := &URLTest{
		myOutboundAdapter: myOutboundAdapter{
			protocol: C.TypeURLTest,
			router:   router,
			logger:   logger,
			tag:      tag,
		},
		tags:      options.Outbounds,
		link:      options.URL,
		interval:  time.Duration(options.Interval),
		tolerance: options.Tolerance,
	}
	if len(outbound.tags) == 0 {
		return nil, E.New("missing tags")
	}
	return outbound, nil
}

func (s *URLTest) Network() []string {
	if s.group == nil {
		return []string{N.NetworkTCP, N.NetworkUDP}
	}
	return s.group.Select(N.NetworkTCP).Network()
}

func (s *URLTest) Start() error {
	outbounds := make([]adapter.Outbound, 0, len(s.tags))
	for i, tag := range s.tags {
		detour, loaded := s.router.Outbound(tag)
		if !loaded {
			return E.New("outbound ", i, " not found: ", tag)
		}
		outbounds = append(outbounds, detour)
	}
	s.group = NewURLTestGroup(s.router, s.logger, outbounds, s.link, s.interval, s.tolerance)
	return s.group.Start()
}

func (s URLTest) Close() error {
	return common.Close(
		common.PtrOrNil(s.group),
	)
}

func (s *URLTest) Now() string {
	return s.group.Select(N.NetworkTCP).Tag()
}

func (s *URLTest) All() []string {
	return s.tags
}

func (s *URLTest) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	outbound := s.group.Select(network)
	conn, err := outbound.DialContext(ctx, network, destination)
	if err == nil {
		return conn, nil
	}
	s.logger.ErrorContext(ctx, err)
	go s.group.checkOutbounds()
	outbounds := s.group.Fallback(outbound)
	for _, fallback := range outbounds {
		conn, err = fallback.DialContext(ctx, network, destination)
		if err == nil {
			return conn, nil
		}
	}
	return nil, err
}

func (s *URLTest) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	outbound := s.group.Select(N.NetworkUDP)
	conn, err := outbound.ListenPacket(ctx, destination)
	if err == nil {
		return conn, nil
	}
	s.logger.ErrorContext(ctx, err)
	go s.group.checkOutbounds()
	outbounds := s.group.Fallback(outbound)
	for _, fallback := range outbounds {
		conn, err = fallback.ListenPacket(ctx, destination)
		if err == nil {
			return conn, nil
		}
	}
	return nil, err
}

func (s *URLTest) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return NewConnection(ctx, s, conn, metadata)
}

func (s *URLTest) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return NewPacketConnection(ctx, s, conn, metadata)
}

type URLTestGroup struct {
	router    adapter.Router
	logger    log.Logger
	outbounds []adapter.Outbound
	link      string
	interval  time.Duration
	tolerance uint16
	history   *urltest.HistoryStorage

	ticker *time.Ticker
	close  chan struct{}
}

func NewURLTestGroup(router adapter.Router, logger log.Logger, outbounds []adapter.Outbound, link string, interval time.Duration, tolerance uint16) *URLTestGroup {
	if link == "" {
		//goland:noinspection HttpUrlsUsage
		link = "http://www.gstatic.com/generate_204"
	}
	if interval == 0 {
		interval = C.DefaultURLTestInterval
	}
	if tolerance == 0 {
		tolerance = 50
	}
	var history *urltest.HistoryStorage
	if clashServer := router.ClashServer(); clashServer != nil {
		history = clashServer.HistoryStorage()
	} else {
		history = urltest.NewHistoryStorage()
	}
	return &URLTestGroup{
		router:    router,
		logger:    logger,
		outbounds: outbounds,
		link:      link,
		interval:  interval,
		tolerance: tolerance,
		history:   history,
		close:     make(chan struct{}),
	}
}

func (g *URLTestGroup) Start() error {
	g.ticker = time.NewTicker(g.interval)
	go g.loopCheck()
	return nil
}

func (g *URLTestGroup) Close() error {
	g.ticker.Stop()
	close(g.close)
	return nil
}

func (g *URLTestGroup) Select(network string) adapter.Outbound {
	var minDelay uint16
	var minTime time.Time
	var minOutbound adapter.Outbound
	for _, detour := range g.outbounds {
		if !common.Contains(detour.Network(), network) {
			continue
		}
		history := g.history.LoadURLTestHistory(RealTag(detour))
		if history == nil {
			continue
		}
		if minDelay == 0 || minDelay > history.Delay+g.tolerance || minDelay > history.Delay-g.tolerance && minTime.Before(history.Time) {
			minDelay = history.Delay
			minTime = history.Time
			minOutbound = detour
		}
	}
	if minOutbound == nil {
		for _, detour := range g.outbounds {
			if !common.Contains(detour.Network(), network) {
				continue
			}
			minOutbound = detour
			break
		}
	}
	return minOutbound
}

func (g *URLTestGroup) Fallback(used adapter.Outbound) []adapter.Outbound {
	outbounds := make([]adapter.Outbound, 0, len(g.outbounds)-1)
	for _, detour := range g.outbounds {
		if detour != used {
			outbounds = append(outbounds, detour)
		}
	}
	sort.Slice(outbounds, func(i, j int) bool {
		oi := outbounds[i]
		oj := outbounds[j]
		hi := g.history.LoadURLTestHistory(RealTag(oi))
		if hi == nil {
			return false
		}
		hj := g.history.LoadURLTestHistory(RealTag(oj))
		if hj == nil {
			return false
		}
		return hi.Delay < hj.Delay
	})
	return outbounds
}

func (g *URLTestGroup) loopCheck() {
	go g.checkOutbounds()
	for {
		select {
		case <-g.close:
			return
		case <-g.ticker.C:
			g.checkOutbounds()
		}
	}
}

func (g *URLTestGroup) checkOutbounds() {
	b, _ := batch.New(context.Background(), batch.WithConcurrencyNum[any](10))
	checked := make(map[string]bool)
	for _, detour := range g.outbounds {
		tag := detour.Tag()
		realTag := RealTag(detour)
		if checked[realTag] {
			continue
		}
		history := g.history.LoadURLTestHistory(realTag)
		if history != nil && time.Now().Sub(history.Time) < g.interval {
			continue
		}
		checked[realTag] = true
		p, loaded := g.router.Outbound(realTag)
		if !loaded {
			continue
		}
		b.Go(realTag, func() (any, error) {
			ctx, cancel := context.WithTimeout(context.Background(), C.TCPTimeout)
			defer cancel()
			t, err := urltest.URLTest(ctx, g.link, p)
			if err != nil {
				g.logger.Debug("outbound ", tag, " unavailable: ", err)
				g.history.DeleteURLTestHistory(realTag)
			} else {
				g.logger.Debug("outbound ", tag, " available: ", t, "ms")
				g.history.StoreURLTestHistory(realTag, &urltest.History{
					Time:  time.Now(),
					Delay: t,
				})
			}
			return nil, nil
		})
	}
	b.Wait()
}
