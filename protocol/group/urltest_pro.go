package group

import (
	"context"
	"fmt"
	"math"
	"net"
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

// RegisterURLTestPro 注册 urltest_pro 协议
// 输入：registry - outbound 注册表
func RegisterURLTestPro(registry *outbound.Registry) {
	outbound.Register[option.URLTestProOutboundOptions](registry, C.TypeURLTestPro, NewURLTestPro)
}

var _ adapter.OutboundGroup = (*URLTestPro)(nil)

// URLTestPro 带权重的自动选择 outbound
// 选择公式：score = delay / weight，选择分数最小的节点
type URLTestPro struct {
	outbound.Adapter
	ctx                          context.Context
	router                       adapter.Router
	outbound                     adapter.OutboundManager
	connection                   adapter.ConnectionManager
	logger                       log.ContextLogger
	tags                         []string
	link                         string
	interval                     time.Duration
	tolerance                    uint16
	idleTimeout                  time.Duration
	group                        *URLTestProGroup
	interruptExternalConnections bool
	weightStorage                adapter.OutboundWeightStorage
}

// NewURLTestPro 创建 URLTestPro 实例
// 输入：
//   - ctx: 上下文
//   - router: 路由器
//   - logger: 日志器
//   - tag: 标签
//   - options: 配置选项
//
// 输出：Outbound 实例和错误
func NewURLTestPro(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.URLTestProOutboundOptions) (adapter.Outbound, error) {
	outbound := &URLTestPro{
		Adapter:                      outbound.NewAdapter(C.TypeURLTestPro, tag, []string{N.NetworkTCP, N.NetworkUDP}, options.Outbounds),
		ctx:                          ctx,
		router:                       router,
		outbound:                     service.FromContext[adapter.OutboundManager](ctx),
		connection:                   service.FromContext[adapter.ConnectionManager](ctx),
		logger:                       logger,
		tags:                         options.Outbounds,
		link:                         options.URL,
		interval:                     time.Duration(options.Interval),
		tolerance:                    options.Tolerance,
		idleTimeout:                  time.Duration(options.IdleTimeout),
		interruptExternalConnections: options.InterruptExistConnections,
		weightStorage:                service.FromContext[adapter.OutboundWeightStorage](ctx),
	}
	if len(outbound.tags) == 0 {
		return nil, E.New("missing tags")
	}
	return outbound, nil
}

// Start 启动 URLTestPro
// 获取各节点权重，weight=0 的节点将被禁用
func (s *URLTestPro) Start() error {
	outbounds := make([]adapter.Outbound, 0, len(s.tags))
	weights := make(map[string]float64)

	for i, tag := range s.tags {
		detour, loaded := s.outbound.Outbound(tag)
		if !loaded {
			return E.New("outbound ", i, " not found: ", tag)
		}

		// 获取权重，默认为 1.0
		weight := 1.0
		if s.weightStorage != nil {
			if w, exists := s.weightStorage.LoadWeight(tag); exists {
				weight = w
			}
		}

		// weight = 0 表示禁用该节点
		if weight == 0 {
			s.logger.Debug("outbound ", tag, " is disabled (weight=0)")
			continue
		}

		weights[tag] = weight
		outbounds = append(outbounds, detour)
	}

	if len(outbounds) == 0 {
		return E.New("no available outbounds (all disabled or weight=0)")
	}

	group, err := NewURLTestProGroup(
		s.ctx,
		s.outbound,
		s.logger,
		outbounds,
		weights,
		s.link,
		s.interval,
		s.tolerance,
		s.idleTimeout,
		s.interruptExternalConnections,
	)
	if err != nil {
		return err
	}
	s.group = group
	return nil
}

func (s *URLTestPro) PostStart() error {
	s.group.PostStart()
	return nil
}

func (s *URLTestPro) Close() error {
	return common.Close(
		common.PtrOrNil(s.group),
	)
}

func (s *URLTestPro) Now() string {
	if s.group.selectedOutboundTCP != nil {
		return s.group.selectedOutboundTCP.Tag()
	} else if s.group.selectedOutboundUDP != nil {
		return s.group.selectedOutboundUDP.Tag()
	}
	return ""
}

func (s *URLTestPro) All() []string {
	return s.tags
}

func (s *URLTestPro) URLTest(ctx context.Context) (map[string]uint16, error) {
	return s.group.URLTest(ctx)
}

func (s *URLTestPro) CheckOutbounds() {
	s.group.CheckOutbounds(true)
}

func (s *URLTestPro) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
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

func (s *URLTestPro) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
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

func (s *URLTestPro) NewConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	ctx = interrupt.ContextWithIsExternalConnection(ctx)
	s.connection.NewConnection(ctx, s, conn, metadata, onClose)
}

func (s *URLTestPro) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	ctx = interrupt.ContextWithIsExternalConnection(ctx)
	s.connection.NewPacketConnection(ctx, s, conn, metadata, onClose)
}

// URLTestProGroup 带权重的测试组
type URLTestProGroup struct {
	ctx                          context.Context
	router                       adapter.Router
	outbound                     adapter.OutboundManager
	pause                        pause.Manager
	pauseCallback                *list.Element[pause.Callback]
	logger                       log.Logger
	outbounds                    []adapter.Outbound
	weights                      map[string]float64 // 节点权重
	link                         string
	interval                     time.Duration
	tolerance                    uint16
	idleTimeout                  time.Duration
	history                      adapter.URLTestHistoryStorage
	checking                     atomic.Bool
	selectedOutboundTCP          adapter.Outbound
	selectedOutboundUDP          adapter.Outbound
	interruptGroup               *interrupt.Group
	interruptExternalConnections bool
	access                       sync.Mutex
	ticker                       *time.Ticker
	close                        chan struct{}
	started                      bool
	lastActive                   common.TypedValue[time.Time]
}

// NewURLTestProGroup 创建测试组
// 输入：
//   - ctx: 上下文
//   - outboundManager: outbound 管理器
//   - logger: 日志器
//   - outbounds: outbound 列表
//   - weights: 权重映射
//   - link: 测试 URL
//   - interval: 测试间隔
//   - tolerance: 分数容差
//   - idleTimeout: 空闲超时
//   - interruptExternalConnections: 是否中断外部连接
//
// 输出：测试组实例和错误
func NewURLTestProGroup(
	ctx context.Context,
	outboundManager adapter.OutboundManager,
	logger log.Logger,
	outbounds []adapter.Outbound,
	weights map[string]float64,
	link string,
	interval time.Duration,
	tolerance uint16,
	idleTimeout time.Duration,
	interruptExternalConnections bool,
) (*URLTestProGroup, error) {
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
	var history adapter.URLTestHistoryStorage
	if historyFromCtx := service.PtrFromContext[urltest.HistoryStorage](ctx); historyFromCtx != nil {
		history = historyFromCtx
	} else if clashServer := service.FromContext[adapter.ClashServer](ctx); clashServer != nil {
		history = clashServer.HistoryStorage()
	} else {
		history = urltest.NewHistoryStorage()
	}
	return &URLTestProGroup{
		ctx:                          ctx,
		outbound:                     outboundManager,
		logger:                       logger,
		outbounds:                    outbounds,
		weights:                      weights,
		link:                         link,
		interval:                     interval,
		tolerance:                    tolerance,
		idleTimeout:                  idleTimeout,
		history:                      history,
		close:                        make(chan struct{}),
		pause:                        service.FromContext[pause.Manager](ctx),
		interruptGroup:               interrupt.NewGroup(),
		interruptExternalConnections: interruptExternalConnections,
	}, nil
}

func (g *URLTestProGroup) PostStart() {
	g.access.Lock()
	defer g.access.Unlock()
	g.started = true
	g.lastActive.Store(time.Now())
	go g.CheckOutbounds(false)
}

func (g *URLTestProGroup) Touch() {
	if !g.started {
		return
	}
	g.access.Lock()
	defer g.access.Unlock()
	if g.ticker != nil {
		g.lastActive.Store(time.Now())
		return
	}
	g.ticker = time.NewTicker(g.interval)
	go g.loopCheck()
	g.pauseCallback = pause.RegisterTicker(g.pause, g.ticker, g.interval, nil)
}

func (g *URLTestProGroup) Close() error {
	g.access.Lock()
	defer g.access.Unlock()
	if g.ticker == nil {
		return nil
	}
	g.ticker.Stop()
	g.pause.UnregisterCallback(g.pauseCallback)
	close(g.close)
	return nil
}

// Select 选择最优节点
// 核心算法：score = delay / weight，选择分数最小的节点
// 输入：network - 网络类型 (tcp/udp)
// 输出：选中的 outbound 和是否基于历史数据选择
func (g *URLTestProGroup) Select(network string) (adapter.Outbound, bool) {
	var minScore float64 = math.MaxFloat64
	var minOutbound adapter.Outbound

	// 检查当前选中的节点
	switch network {
	case N.NetworkTCP:
		if g.selectedOutboundTCP != nil {
			if history := g.history.LoadURLTestHistory(RealTag(g.selectedOutboundTCP)); history != nil {
				weight := g.getWeight(g.selectedOutboundTCP.Tag())
				if weight > 0 {
					minScore = float64(history.Delay) / weight
					minOutbound = g.selectedOutboundTCP
				}
			}
		}
	case N.NetworkUDP:
		if g.selectedOutboundUDP != nil {
			if history := g.history.LoadURLTestHistory(RealTag(g.selectedOutboundUDP)); history != nil {
				weight := g.getWeight(g.selectedOutboundUDP.Tag())
				if weight > 0 {
					minScore = float64(history.Delay) / weight
					minOutbound = g.selectedOutboundUDP
				}
			}
		}
	}

	// 遍历所有节点，计算分数
	for _, detour := range g.outbounds {
		if !common.Contains(detour.Network(), network) {
			continue
		}

		history := g.history.LoadURLTestHistory(RealTag(detour))
		if history == nil {
			continue
		}

		weight := g.getWeight(detour.Tag())
		if weight <= 0 {
			continue // 跳过禁用的节点
		}

		// 计算分数：score = delay / weight
		score := float64(history.Delay) / weight

		// 应用容差机制到分数
		// 只有当新分数显著优于当前分数时才切换
		toleranceScore := float64(g.tolerance)
		if minScore == math.MaxFloat64 || minScore > score+toleranceScore {
			minScore = score
			minOutbound = detour
		}
	}

	if minOutbound == nil {
		// 如果没有历史数据，返回第一个可用的
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

// getWeight 获取节点权重
// 输入：tag - 节点标签
// 输出：权重值（默认 1.0）
func (g *URLTestProGroup) getWeight(tag string) float64 {
	if weight, ok := g.weights[tag]; ok {
		return weight
	}
	return 1.0
}

func (g *URLTestProGroup) loopCheck() {
	if time.Since(g.lastActive.Load()) > g.interval {
		g.lastActive.Store(time.Now())
		g.CheckOutbounds(false)
	}
	for {
		select {
		case <-g.close:
			return
		case <-g.ticker.C:
		}
		if time.Since(g.lastActive.Load()) > g.idleTimeout {
			g.access.Lock()
			g.ticker.Stop()
			g.ticker = nil
			g.pause.UnregisterCallback(g.pauseCallback)
			g.pauseCallback = nil
			g.access.Unlock()
			return
		}
		g.CheckOutbounds(false)
	}
}

func (g *URLTestProGroup) CheckOutbounds(force bool) {
	_, _ = g.urlTest(g.ctx, force)
}

func (g *URLTestProGroup) URLTest(ctx context.Context) (map[string]uint16, error) {
	return g.urlTest(ctx, false)
}

func (g *URLTestProGroup) urlTest(ctx context.Context, force bool) (map[string]uint16, error) {
	result := make(map[string]uint16)
	if g.checking.Swap(true) {
		return result, nil
	}
	defer g.checking.Store(false)
	b, _ := batch.New(ctx, batch.WithConcurrencyNum[any](10))
	checked := make(map[string]bool)
	var resultAccess sync.Mutex
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
				weight := g.getWeight(tag)
				score := float64(t) / weight
				g.logger.Debug("outbound ", tag, " available: ", t, "ms, weight: ", fmt.Sprintf("%.2f", weight), ", score: ", fmt.Sprintf("%.2f", score))
				g.history.StoreURLTestHistory(realTag, &adapter.URLTestHistory{
					Time:  time.Now(),
					Delay: t,
				})
				resultAccess.Lock()
				result[tag] = t
				resultAccess.Unlock()
			}
			return nil, nil
		})
	}
	b.Wait()
	g.performUpdateCheck()
	return result, nil
}

func (g *URLTestProGroup) performUpdateCheck() {
	var updated bool
	if outbound, exists := g.Select(N.NetworkTCP); outbound != nil && (g.selectedOutboundTCP == nil || (exists && outbound != g.selectedOutboundTCP)) {
		if g.selectedOutboundTCP != nil {
			updated = true
		}
		g.selectedOutboundTCP = outbound
	}
	if outbound, exists := g.Select(N.NetworkUDP); outbound != nil && (g.selectedOutboundUDP == nil || (exists && outbound != g.selectedOutboundUDP)) {
		if g.selectedOutboundUDP != nil {
			updated = true
		}
		g.selectedOutboundUDP = outbound
	}
	if updated {
		g.interruptGroup.Interrupt(g.interruptExternalConnections)
	}
}
