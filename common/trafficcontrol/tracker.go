package trafficcontrol

import (
	"context"
	"net"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	N "github.com/sagernet/sing/common/network"

	"github.com/gofrs/uuid/v5"
)

type TrackerMetadata struct {
	ID           uuid.UUID
	Metadata     adapter.InboundContext
	CreatedAt    time.Time
	ClosedAt     time.Time
	Upload       *atomic.Int64
	Download     *atomic.Int64
	Chain        []string
	Rule         adapter.Rule
	Outbound     string
	OutboundType string
}

type Tracker interface {
	Metadata() *TrackerMetadata
	Close() error
}

func (m *Manager) RoutedConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) net.Conn {
	upload := new(atomic.Int64)
	download := new(atomic.Int64)
	tracker := &connTracker{
		ExtendedConn: bufio.NewCounterConn(conn, []N.CountFunc{func(n int64) {
			upload.Add(n)
			m.uploadTotal.Add(n)
		}}, []N.CountFunc{func(n int64) {
			download.Add(n)
			m.downloadTotal.Add(n)
		}}),
		metadata: m.newTrackerMetadata(metadata, matchedRule, matchOutbound, upload, download),
		manager:  m,
	}
	m.join(tracker)
	return tracker
}

func (m *Manager) RoutedPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) N.PacketConn {
	upload := new(atomic.Int64)
	download := new(atomic.Int64)
	tracker := &packetConnTracker{
		PacketConn: bufio.NewCounterPacketConn(conn, []N.CountFunc{func(n int64) {
			upload.Add(n)
			m.uploadTotal.Add(n)
		}}, []N.CountFunc{func(n int64) {
			download.Add(n)
			m.downloadTotal.Add(n)
		}}),
		metadata: m.newTrackerMetadata(metadata, matchedRule, matchOutbound, upload, download),
		manager:  m,
	}
	m.join(tracker)
	return tracker
}

func (m *Manager) newTrackerMetadata(metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound, upload *atomic.Int64, download *atomic.Int64) TrackerMetadata {
	id, _ := uuid.NewV4()
	var (
		chain        []string
		next         string
		outbound     string
		outboundType string
	)
	if matchOutbound != nil {
		next = matchOutbound.Tag()
	} else {
		next = m.outbound.Default().Tag()
	}
	for {
		detour, loaded := m.outbound.Outbound(next)
		if !loaded {
			break
		}
		chain = append(chain, next)
		outbound = detour.Tag()
		outboundType = detour.Type()
		outboundGroup, isGroup := detour.(adapter.OutboundGroup)
		if !isGroup {
			break
		}
		next = outboundGroup.Now()
	}
	return TrackerMetadata{
		ID:           id,
		Metadata:     metadata,
		CreatedAt:    time.Now(),
		Upload:       upload,
		Download:     download,
		Chain:        common.Reverse(chain),
		Rule:         matchedRule,
		Outbound:     outbound,
		OutboundType: outboundType,
	}
}

type connTracker struct {
	N.ExtendedConn
	metadata TrackerMetadata
	manager  *Manager
}

func (t *connTracker) Metadata() *TrackerMetadata {
	return &t.metadata
}

func (t *connTracker) Close() error {
	t.manager.leave(t)
	return t.ExtendedConn.Close()
}

func (t *connTracker) Upstream() any {
	return t.ExtendedConn
}

func (t *connTracker) ReaderReplaceable() bool {
	return true
}

func (t *connTracker) WriterReplaceable() bool {
	return true
}

type packetConnTracker struct {
	N.PacketConn
	metadata TrackerMetadata
	manager  *Manager
}

func (t *packetConnTracker) Metadata() *TrackerMetadata {
	return &t.metadata
}

func (t *packetConnTracker) Close() error {
	t.manager.leave(t)
	return t.PacketConn.Close()
}

func (t *packetConnTracker) Upstream() any {
	return t.PacketConn
}

func (t *packetConnTracker) ReaderReplaceable() bool {
	return true
}

func (t *packetConnTracker) WriterReplaceable() bool {
	return true
}
