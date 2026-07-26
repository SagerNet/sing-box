package smart

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/interrupt"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/protocol/group/smart/engine"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

// RegisterOutbound registers type "smart" on the outbound registry.
// dart-smart:register
func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register[option.SmartOutboundOptions](registry, C.TypeSmart, NewOutbound)
}

var (
	_ adapter.OutboundGroup             = (*Outbound)(nil)
	_ adapter.ConnectionHandlerEx       = (*Outbound)(nil)
	_ adapter.PacketConnectionHandlerEx = (*Outbound)(nil)
)

// Outbound is a Smart group: ordered dial attempts with connection-level failover.
type Outbound struct {
	outbound.Adapter
	ctx                          context.Context
	outboundManager              adapter.OutboundManager
	connection                   adapter.ConnectionManager
	logger                       log.ContextLogger
	tags                         []string
	outbounds                    map[string]adapter.Outbound
	engine                       *engine.Engine
	selected                     string
	interruptGroup               *interrupt.Group
	interruptExternalConnections bool
}

func NewOutbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.SmartOutboundOptions) (adapter.Outbound, error) {
	if len(options.Outbounds) == 0 {
		return nil, E.New("missing outbounds")
	}
	return &Outbound{
		Adapter:                      outbound.NewAdapter(C.TypeSmart, tag, []string{N.NetworkTCP, N.NetworkUDP}, options.Outbounds),
		ctx:                          ctx,
		outboundManager:              service.FromContext[adapter.OutboundManager](ctx),
		connection:                   service.FromContext[adapter.ConnectionManager](ctx),
		logger:                       logger,
		tags:                         options.Outbounds,
		outbounds:                    make(map[string]adapter.Outbound),
		engine:                       engine.New(options.Outbounds, engine.OptionsForMode(options.Mode)),
		interruptGroup:               interrupt.NewGroup(),
		interruptExternalConnections: options.InterruptExistConnections,
	}, nil
}

func (s *Outbound) Start() error {
	for i, tag := range s.tags {
		detour, loaded := s.outboundManager.Outbound(tag)
		if !loaded {
			return E.New("outbound ", i, " not found: ", tag)
		}
		s.outbounds[tag] = detour
	}
	s.selected = s.tags[0]
	return nil
}

func (s *Outbound) Close() error {
	return nil
}

func (s *Outbound) Now() string {
	if s.selected != "" {
		return s.selected
	}
	if len(s.tags) > 0 {
		return s.tags[0]
	}
	return ""
}

func (s *Outbound) All() []string {
	return s.tags
}

// SelectOutbound pins the preferred member for subsequent dials (Clash API).
// Failover still walks other members when the preferred dial fails.
func (s *Outbound) SelectOutbound(tag string) bool {
	if _, ok := s.outbounds[tag]; !ok {
		return false
	}
	s.selected = tag
	if s.engine != nil {
		s.engine.SetPreferred(tag)
	}
	if s.Tag() != "" {
		cacheFile := service.FromContext[adapter.CacheFile](s.ctx)
		if cacheFile != nil {
			_ = cacheFile.StoreSelected(s.Tag(), tag)
		}
	}
	return true
}

func (s *Outbound) recordDialFeedback(
	tag string,
	signal string,
	legacy bool,
	success bool,
	duration time.Duration,
	errorClass string,
) {
	network := "tcp"
	if signal == dialFeedbackSignalUDP {
		network = "udp"
	}
	recordDialFeedback(s.Tag(), tag, network, signal, legacy, success, duration, errorClass)
}

func (s *Outbound) recordHandshakeFeedback(
	tag string,
	success bool,
	writeElapsed time.Duration,
	totalElapsed time.Duration,
	errorClass string,
) {
	recordProjectedDialFeedback(
		s.Tag(),
		tag,
		"tcp",
		dialFeedbackSignalHandshake,
		true,
		success,
		writeElapsed,
		totalElapsed,
		errorClass,
	)
}

func (s *Outbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	host := destination.Fqdn
	if host == "" {
		host = destination.AddrString()
	}
	var lastErr error
	for _, candidate := range s.engine.SelectFor(host, engine.NetworkTCP) {
		detour := s.outbounds[candidate.Tag]
		if detour == nil {
			continue
		}
		if !common.Contains(detour.Network(), N.NetworkName(network)) {
			continue
		}
		start := time.Now()
		conn, err := detour.DialContext(ctx, network, destination)
		elapsed := time.Since(start)
		rttMs := float64(elapsed.Milliseconds())
		if err != nil {
			s.engine.RecordFor(host, engine.NetworkTCP, candidate.Tag, engine.OutcomeFailure, rttMs)
			s.recordDialFeedback(
				candidate.Tag,
				dialFeedbackSignalTCP,
				true,
				false,
				elapsed,
				classifyDialError(err),
			)
			s.logger.DebugContext(ctx, "smart dial failed via ", candidate.Tag, ": ", err)
			lastErr = err
			continue
		}
		// Soft-fail: dial itself was slow vs history.
		threshold := s.engine.SoftFailThresholdMs(candidate.Tag)
		if rttMs > threshold && threshold > 0 {
			_ = conn.Close()
			s.engine.RecordFor(host, engine.NetworkTCP, candidate.Tag, engine.OutcomeSoftFail, rttMs)
			s.recordDialFeedback(
				candidate.Tag,
				dialFeedbackSignalTCP,
				true,
				false,
				elapsed,
				"soft-fail",
			)
			s.logger.DebugContext(ctx, "smart soft-fail via ", candidate.Tag, " rtt=", int(rttMs), "ms threshold=", int(threshold), "ms")
			lastErr = E.New("smart soft-fail: ", candidate.Tag)
			continue
		}
		tag := candidate.Tag
		hostKey := host
		needsHandshake := N.NeedHandshakeForWrite(conn)
		s.recordDialFeedback(
			tag,
			dialFeedbackSignalTCP,
			!needsHandshake,
			true,
			elapsed,
			"",
		)
		// Defer success until first write when the outbound still needs a handshake.
		if needsHandshake {
			conn = &firstWriteObserveConn{
				Conn: conn,
				onFirstWrite: func(err error, writeElapsed time.Duration) {
					hsMs := float64(writeElapsed.Milliseconds())
					totalElapsed := elapsed + writeElapsed
					if err != nil {
						s.engine.RecordFor(hostKey, engine.NetworkTCP, tag, engine.OutcomeFailure, hsMs)
						s.recordHandshakeFeedback(
							tag,
							false,
							writeElapsed,
							totalElapsed,
							classifyDialError(err),
						)
						s.logger.DebugContext(ctx, "smart handshake failed via ", tag, ": ", err)
						return
					}
					th := s.engine.SoftFailThresholdMs(tag)
					if hsMs > th && th > 0 {
						s.engine.RecordFor(hostKey, engine.NetworkTCP, tag, engine.OutcomeSoftFail, hsMs)
						s.recordHandshakeFeedback(
							tag,
							false,
							writeElapsed,
							totalElapsed,
							"soft-fail",
						)
						s.logger.DebugContext(ctx, "smart handshake soft-fail via ", tag, " rtt=", int(hsMs), "ms")
						return
					}
					s.engine.RecordFor(hostKey, engine.NetworkTCP, tag, engine.OutcomeSuccess, hsMs)
					s.engine.RememberHostFor(hostKey, engine.NetworkTCP, tag)
					s.recordHandshakeFeedback(
						tag,
						true,
						writeElapsed,
						totalElapsed,
						"",
					)
				},
			}
			s.selected = tag
			conn = s.observeFirstByte(conn, hostKey, tag)
			return s.interruptGroup.NewConn(conn, interrupt.IsExternalConnectionFromContext(ctx)), nil
		}
		s.engine.RecordFor(host, engine.NetworkTCP, candidate.Tag, engine.OutcomeSuccess, rttMs)
		s.engine.RememberHostFor(host, engine.NetworkTCP, candidate.Tag)
		s.selected = candidate.Tag
		conn = s.observeFirstByte(conn, host, candidate.Tag)
		return s.interruptGroup.NewConn(conn, interrupt.IsExternalConnectionFromContext(ctx)), nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, E.New("smart: no usable outbound")
}

func (s *Outbound) observeFirstByte(conn net.Conn, host, tag string) net.Conn {
	return &firstByteObserveConn{
		ExtendedConn: bufio.NewExtendedConn(conn),
		onFirstByte: func(err error, latency time.Duration) {
			latencyMs := float64(latency.Milliseconds())
			s.engine.RecordFirstByteFor(host, engine.NetworkTCP, tag, err == nil, latencyMs)
			s.recordDialFeedback(
				tag,
				dialFeedbackSignalFirstByte,
				false,
				err == nil,
				latency,
				classifyDialError(err),
			)
		},
	}
}

func (s *Outbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	host := destination.Fqdn
	if host == "" {
		host = destination.AddrString()
	}
	var lastErr error
	for _, candidate := range s.engine.SelectFor(host, engine.NetworkUDP) {
		detour := s.outbounds[candidate.Tag]
		if detour == nil {
			continue
		}
		if !common.Contains(detour.Network(), N.NetworkUDP) {
			continue
		}
		start := time.Now()
		conn, err := detour.ListenPacket(ctx, destination)
		elapsed := time.Since(start)
		rttMs := float64(elapsed.Milliseconds())
		if err != nil {
			s.engine.RecordFor(host, engine.NetworkUDP, candidate.Tag, engine.OutcomeFailure, rttMs)
			s.recordDialFeedback(
				candidate.Tag,
				dialFeedbackSignalUDP,
				true,
				false,
				elapsed,
				classifyDialError(err),
			)
			lastErr = err
			continue
		}
		s.engine.RecordFor(host, engine.NetworkUDP, candidate.Tag, engine.OutcomeSuccess, rttMs)
		s.engine.RememberHostFor(host, engine.NetworkUDP, candidate.Tag)
		s.recordDialFeedback(
			candidate.Tag,
			dialFeedbackSignalUDP,
			true,
			true,
			elapsed,
			"",
		)
		s.selected = candidate.Tag
		return s.interruptGroup.NewPacketConn(conn, interrupt.IsExternalConnectionFromContext(ctx)), nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, E.New("smart: no usable outbound for packet")
}

func classifyDialError(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	}
	var networkError net.Error
	if errors.As(err, &networkError) && networkError.Timeout() {
		return "timeout"
	}
	return "network"
}

func (s *Outbound) NewConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	ctx = interrupt.ContextWithIsExternalConnection(ctx)
	s.connection.NewConnection(ctx, s, conn, metadata, onClose)
}

func (s *Outbound) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	ctx = interrupt.ContextWithIsExternalConnection(ctx)
	s.connection.NewPacketConnection(ctx, s, conn, metadata, onClose)
}

// firstWriteObserveConn records handshake outcome on the first Write.
type firstWriteObserveConn struct {
	net.Conn
	once         sync.Once
	onFirstWrite func(err error, latency time.Duration)
}

func (c *firstWriteObserveConn) Write(b []byte) (int, error) {
	isFirstWrite := false
	c.once.Do(func() {
		isFirstWrite = true
	})
	if !isFirstWrite {
		return c.Conn.Write(b)
	}
	start := time.Now()
	n, err := c.Conn.Write(b)
	elapsed := time.Since(start)
	if c.onFirstWrite != nil {
		c.onFirstWrite(err, elapsed)
	}
	return n, err
}

// firstByteObserveConn measures from the first request write to the first read.
// It observes only; replay after a partial write remains deliberately disabled.
type firstByteObserveConn struct {
	N.ExtendedConn
	armOnce     sync.Once
	requestAt   time.Time
	observed    atomic.Bool
	onFirstByte func(err error, latency time.Duration)
}

func (c *firstByteObserveConn) arm() {
	c.armOnce.Do(func() {
		c.requestAt = time.Now()
	})
}

func (c *firstByteObserveConn) elapsed() time.Duration {
	c.arm()
	return time.Since(c.requestAt)
}

func (c *firstByteObserveConn) Write(b []byte) (int, error) {
	c.arm()
	return c.ExtendedConn.Write(b)
}

func (c *firstByteObserveConn) Read(b []byte) (int, error) {
	c.arm()
	n, err := c.ExtendedConn.Read(b)
	if n > 0 || err != nil {
		observedErr := err
		if n > 0 {
			observedErr = nil
		}
		c.observe(observedErr)
	}
	return n, err
}

func (c *firstByteObserveConn) WriteBuffer(buffer *buf.Buffer) error {
	c.arm()
	return c.ExtendedConn.WriteBuffer(buffer)
}

func (c *firstByteObserveConn) ReadBuffer(buffer *buf.Buffer) error {
	c.arm()
	before := buffer.Len()
	err := c.ExtendedConn.ReadBuffer(buffer)
	if buffer.Len() > before {
		c.observe(nil)
	} else if err != nil {
		c.observe(err)
	}
	return err
}

func (c *firstByteObserveConn) observe(err error) {
	if c.observed.Swap(true) {
		return
	}
	latency := c.elapsed()
	if c.onFirstByte != nil {
		c.onFirstByte(err, latency)
	}
}
