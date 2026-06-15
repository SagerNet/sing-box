package group

import (
	"context"
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
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/batch"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/pause"
)

func RegisterPriority(registry *outbound.Registry) {
	outbound.Register[option.PriorityOutboundOptions](registry, C.TypePriority, NewPriority)
}

var _ adapter.OutboundGroup = (*Priority)(nil)

type Priority struct {
	outbound.Adapter
	ctx                          context.Context
	outbound                     adapter.OutboundManager
	connection                   adapter.ConnectionManager
	logger                       log.ContextLogger
	tiers                        [][]string
	tags                         []string
	link                         string
	interval                     time.Duration
	tolerance                    uint16
	idleTimeout                  time.Duration
	tierUpChecks                 uint16
	tierDownChecks               uint16
	group                        *PriorityGroup
	interruptExternalConnections bool
}

func NewPriority(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.PriorityOutboundOptions) (adapter.Outbound, error) {
	tags := make([]string, 0)
	for _, tier := range options.Tiers {
		tags = append(tags, tier...)
	}
	outbound := &Priority{
		Adapter:                      outbound.NewAdapter(C.TypePriority, tag, []string{N.NetworkTCP, N.NetworkUDP}, tags),
		ctx:                          ctx,
		outbound:                     service.FromContext[adapter.OutboundManager](ctx),
		connection:                   service.FromContext[adapter.ConnectionManager](ctx),
		logger:                       logger,
		tiers:                        options.Tiers,
		tags:                         tags,
		link:                         options.URL,
		interval:                     time.Duration(options.Interval),
		tolerance:                    options.Tolerance,
		idleTimeout:                  time.Duration(options.IdleTimeout),
		tierUpChecks:                 options.TierUpChecks,
		tierDownChecks:               options.TierDownChecks,
		interruptExternalConnections: options.InterruptExistConnections,
	}
	if len(options.Tiers) == 0 {
		return nil, E.New("missing tiers")
	}
	for i, tier := range options.Tiers {
		if len(tier) == 0 {
			return nil, E.New("empty tier ", i)
		}
	}
	return outbound, nil
}

func (s *Priority) Start() error {
	tiers := make([][]adapter.Outbound, 0, len(s.tiers))
	for tierIndex, tier := range s.tiers {
		outbounds := make([]adapter.Outbound, 0, len(tier))
		for _, tag := range tier {
			detour, loaded := s.outbound.Outbound(tag)
			if !loaded {
				return E.New("tier ", tierIndex, " outbound not found: ", tag)
			}
			outbounds = append(outbounds, detour)
		}
		tiers = append(tiers, outbounds)
	}
	group, err := NewPriorityGroup(s.ctx, s.outbound, s.logger, tiers, s.link, s.interval, s.tolerance, s.idleTimeout, s.tierUpChecks, s.tierDownChecks, s.interruptExternalConnections)
	if err != nil {
		return err
	}
	s.group = group
	return nil
}

func (s *Priority) PostStart() error {
	s.group.PostStart()
	return nil
}

func (s *Priority) Close() error {
	return common.Close(
		common.PtrOrNil(s.group),
	)
}

func (s *Priority) Now() string {
	if s.group.selectedOutboundTCP != nil {
		return s.group.selectedOutboundTCP.Tag()
	} else if s.group.selectedOutboundUDP != nil {
		return s.group.selectedOutboundUDP.Tag()
	}
	return ""
}

func (s *Priority) All() []string {
	return s.tags
}

func (s *Priority) URLTest(ctx context.Context) (map[string]uint16, error) {
	return s.group.URLTest(ctx)
}

func (s *Priority) CheckOutbounds() {
	s.group.CheckOutbounds(true)
}

func (s *Priority) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
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
		outbound = s.group.Select(network)
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

func (s *Priority) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	s.group.Touch()
	outbound := s.group.selectedOutboundUDP
	if outbound == nil {
		outbound = s.group.Select(N.NetworkUDP)
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

func (s *Priority) NewConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	ctx = interrupt.ContextWithIsExternalConnection(ctx)
	s.connection.NewConnection(ctx, s, conn, metadata, onClose)
}

func (s *Priority) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	ctx = interrupt.ContextWithIsExternalConnection(ctx)
	s.connection.NewPacketConnection(ctx, s, conn, metadata, onClose)
}

func (s *Priority) NewDirectRouteConnection(metadata adapter.InboundContext, routeContext tun.DirectRouteContext, timeout time.Duration) (tun.DirectRouteDestination, error) {
	s.group.Touch()
	selected := s.group.selectedOutboundTCP
	if selected == nil {
		selected = s.group.Select(N.NetworkTCP)
	}
	if selected == nil {
		return nil, E.New("missing supported outbound")
	}
	if !common.Contains(selected.Network(), metadata.Network) {
		return nil, E.New(metadata.Network, " is not supported by outbound: ", selected.Tag())
	}
	return selected.(adapter.DirectRouteOutbound).NewDirectRouteConnection(metadata, routeContext, timeout)
}

// tierState holds the per-network priority state machine.
type tierState struct {
	active        int
	healthyStreak []uint16
	deadStreak    uint16
}

type PriorityGroup struct {
	ctx                          context.Context
	outbound                     adapter.OutboundManager
	pause                        pause.Manager
	pauseCallback                *list.Element[pause.Callback]
	logger                       log.Logger
	tiers                        [][]adapter.Outbound
	outbounds                    []adapter.Outbound
	link                         string
	interval                     time.Duration
	tolerance                    uint16
	idleTimeout                  time.Duration
	tierUpChecks                 uint16
	tierDownChecks               uint16
	history                      *urltest.HistoryStorage
	checking                     atomic.Bool
	stateTCP                     tierState
	stateUDP                     tierState
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

func NewPriorityGroup(ctx context.Context, outboundManager adapter.OutboundManager, logger log.Logger, tiers [][]adapter.Outbound, link string, interval time.Duration, tolerance uint16, idleTimeout time.Duration, tierUpChecks uint16, tierDownChecks uint16, interruptExternalConnections bool) (*PriorityGroup, error) {
	if interval == 0 {
		interval = C.DefaultURLTestInterval
	}
	if tolerance == 0 {
		tolerance = 50
	}
	if idleTimeout == 0 {
		idleTimeout = C.DefaultURLTestIdleTimeout
	}
	if tierUpChecks == 0 {
		tierUpChecks = 2
	}
	if tierDownChecks == 0 {
		tierDownChecks = 1
	}
	if interval > idleTimeout {
		return nil, E.New("interval must be less or equal than idle_timeout")
	}
	history := service.PtrFromContext[urltest.HistoryStorage](ctx)
	if history == nil {
		return nil, E.New("missing URL test history storage")
	}
	outbounds := make([]adapter.Outbound, 0)
	for _, tier := range tiers {
		outbounds = append(outbounds, tier...)
	}
	return &PriorityGroup{
		ctx:                          ctx,
		outbound:                     outboundManager,
		logger:                       logger,
		tiers:                        tiers,
		outbounds:                    outbounds,
		link:                         link,
		interval:                     interval,
		tolerance:                    tolerance,
		idleTimeout:                  idleTimeout,
		tierUpChecks:                 tierUpChecks,
		tierDownChecks:               tierDownChecks,
		history:                      history,
		stateTCP:                     tierState{active: -1, healthyStreak: make([]uint16, len(tiers))},
		stateUDP:                     tierState{active: -1, healthyStreak: make([]uint16, len(tiers))},
		close:                        make(chan struct{}),
		pause:                        service.FromContext[pause.Manager](ctx),
		interruptGroup:               interrupt.NewGroup(),
		interruptExternalConnections: interruptExternalConnections,
	}, nil
}

func (g *PriorityGroup) PostStart() {
	g.access.Lock()
	defer g.access.Unlock()
	g.started = true
	g.lastActive.Store(time.Now())
	go g.CheckOutbounds(false)
}

func (g *PriorityGroup) Touch() {
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

func (g *PriorityGroup) Close() error {
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

// tierAlive reports whether at least one outbound in the tier has a live probe
// result and supports the given network.
func (g *PriorityGroup) tierAlive(tierIndex int, network string) bool {
	for _, detour := range g.tiers[tierIndex] {
		if !common.Contains(detour.Network(), network) {
			continue
		}
		if g.history.LoadURLTestHistory(RealTag(detour)) != nil {
			return true
		}
	}
	return false
}

// selectInTier returns the lowest-delay live outbound within the tier for the
// given network, applying tolerance hysteresis against the currently selected
// outbound when it belongs to the same tier.
func (g *PriorityGroup) selectInTier(tierIndex int, network string, current adapter.Outbound) adapter.Outbound {
	var minDelay uint16
	var minOutbound adapter.Outbound
	if current != nil {
		for _, detour := range g.tiers[tierIndex] {
			if detour != current {
				continue
			}
			if history := g.history.LoadURLTestHistory(RealTag(current)); history != nil {
				minOutbound = current
				minDelay = history.Delay
			}
			break
		}
	}
	for _, detour := range g.tiers[tierIndex] {
		if !common.Contains(detour.Network(), network) {
			continue
		}
		history := g.history.LoadURLTestHistory(RealTag(detour))
		if history == nil {
			continue
		}
		if minDelay == 0 || minDelay > history.Delay+g.tolerance {
			minDelay = history.Delay
			minOutbound = detour
		}
	}
	return minOutbound
}

// decideTier advances the priority state machine for one probe round given the
// per-tier liveness vector. It is pure (no I/O) so the transition policy can be
// unit-tested in isolation: hold the active tier while it has any live member;
// drop to the best live tier below only after tierDownChecks consecutive dead
// rounds; climb to a healthier tier above only after tierUpChecks consecutive
// healthy rounds.
func decideTier(state *tierState, alive []bool, tierUpChecks uint16, tierDownChecks uint16) {
	bestAlive := -1
	for i := range alive {
		if alive[i] {
			state.healthyStreak[i]++
			if bestAlive == -1 {
				bestAlive = i
			}
		} else {
			state.healthyStreak[i] = 0
		}
	}

	switch {
	case state.active == -1:
		// Cold start: take the best live tier immediately.
		state.active = bestAlive
		state.deadStreak = 0
	case bestAlive == -1:
		// Nothing reachable anywhere; keep the active tier as-is.
	case bestAlive < state.active:
		// A higher-priority tier is live. Climb only after it has stayed
		// healthy for tierUpChecks consecutive rounds (hysteresis), picking
		// the highest such tier above the active one.
		for i := bestAlive; i < state.active; i++ {
			if alive[i] && state.healthyStreak[i] >= tierUpChecks {
				state.active = i
				break
			}
		}
		if alive[state.active] {
			state.deadStreak = 0
		}
	case !alive[state.active]:
		// The active tier died; drop to the best live tier below it, but
		// only after tierDownChecks consecutive dead rounds.
		state.deadStreak++
		if state.deadStreak >= tierDownChecks {
			state.active = bestAlive
			state.deadStreak = 0
		}
	default:
		// Active tier still has a live member and is the best available.
		state.deadStreak = 0
	}
}

// updateTier advances the per-network tier state machine: hold the active tier
// while it has any live member; drop to the best live tier below only after
// tierDownChecks consecutive dead rounds; climb to a healthier tier above only
// after tierUpChecks consecutive healthy rounds. It returns the resolved
// outbound for the network, or nil when nothing is reachable.
func (g *PriorityGroup) updateTier(state *tierState, network string) adapter.Outbound {
	alive := make([]bool, len(g.tiers))
	bestAlive := -1
	for i := range g.tiers {
		alive[i] = g.tierAlive(i, network)
		if alive[i] && bestAlive == -1 {
			bestAlive = i
		}
	}
	decideTier(state, alive, g.tierUpChecks, g.tierDownChecks)

	if state.active == -1 {
		return nil
	}
	current := g.selectedOutboundTCP
	if network == N.NetworkUDP {
		current = g.selectedOutboundUDP
	}
	if selected := g.selectInTier(state.active, network, current); selected != nil {
		return selected
	}
	// The active tier has no live member yet (held during a drop delay): fall
	// back to the global best live tier so traffic still flows.
	if bestAlive != -1 {
		return g.selectInTier(bestAlive, network, current)
	}
	return nil
}

// Select returns an outbound for the network without advancing the state
// machine. It is used as a dial-time fallback before the first probe round
// has populated the selected outbounds.
func (g *PriorityGroup) Select(network string) adapter.Outbound {
	for i := range g.tiers {
		if selected := g.selectInTier(i, network, nil); selected != nil {
			return selected
		}
	}
	for _, detour := range g.outbounds {
		if common.Contains(detour.Network(), network) {
			return detour
		}
	}
	return nil
}

func (g *PriorityGroup) loopCheck(ticker *time.Ticker, closeChan <-chan struct{}) {
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

func (g *PriorityGroup) CheckOutbounds(force bool) {
	_, _ = g.urlTest(g.ctx, force)
}

func (g *PriorityGroup) URLTest(ctx context.Context) (map[string]uint16, error) {
	return g.urlTest(ctx, false)
}

func (g *PriorityGroup) urlTest(ctx context.Context, force bool) (map[string]uint16, error) {
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
				g.logger.Debug("outbound ", tag, " available: ", t, "ms")
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

func (g *PriorityGroup) performUpdateCheck() {
	g.access.Lock()
	defer g.access.Unlock()
	var updated bool
	if outbound := g.updateTier(&g.stateTCP, N.NetworkTCP); outbound != nil && outbound != g.selectedOutboundTCP {
		if g.selectedOutboundTCP != nil {
			updated = true
		}
		g.selectedOutboundTCP = outbound
	}
	if outbound := g.updateTier(&g.stateUDP, N.NetworkUDP); outbound != nil && outbound != g.selectedOutboundUDP {
		if g.selectedOutboundUDP != nil {
			updated = true
		}
		g.selectedOutboundUDP = outbound
	}
	if updated {
		g.interruptGroup.Interrupt(g.interruptExternalConnections)
	}
}
