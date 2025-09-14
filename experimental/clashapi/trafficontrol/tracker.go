package trafficontrol

import (
	"net"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/json"
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

func (t TrackerMetadata) MarshalJSON() ([]byte, error) {
	var inbound string
	if t.Metadata.Inbound != "" {
		inbound = t.Metadata.InboundType + "/" + t.Metadata.Inbound
	} else {
		inbound = t.Metadata.InboundType
	}
	var domain string
	if t.Metadata.Domain != "" {
		domain = t.Metadata.Domain
	} else {
		domain = t.Metadata.Destination.Fqdn
	}
	var processPath string
	if t.Metadata.ProcessInfo != nil {
		if t.Metadata.ProcessInfo.ProcessPath != "" {
			processPath = t.Metadata.ProcessInfo.ProcessPath
		} else if t.Metadata.ProcessInfo.PackageName != "" {
			processPath = t.Metadata.ProcessInfo.PackageName
		}
		if processPath == "" {
			if t.Metadata.ProcessInfo.UserId != -1 {
				processPath = F.ToString(t.Metadata.ProcessInfo.UserId)
			}
		} else if t.Metadata.ProcessInfo.User != "" {
			processPath = F.ToString(processPath, " (", t.Metadata.ProcessInfo.User, ")")
		} else if t.Metadata.ProcessInfo.UserId != -1 {
			processPath = F.ToString(processPath, " (", t.Metadata.ProcessInfo.UserId, ")")
		}
	}
	var rule string
	if t.Rule != nil {
		rule = F.ToString(t.Rule, " => ", t.Rule.Action())
	} else {
		rule = "final"
	}
	return json.Marshal(map[string]any{
		"id": t.ID,
		"metadata": map[string]any{
			"network":         t.Metadata.Network,
			"type":            inbound,
			"sourceIP":        t.Metadata.Source.Addr,
			"destinationIP":   t.Metadata.Destination.Addr,
			"sourcePort":      F.ToString(t.Metadata.Source.Port),
			"destinationPort": F.ToString(t.Metadata.Destination.Port),
			"host":            domain,
			"dnsMode":         "normal",
			"processPath":     processPath,
		},
		"upload":      t.Upload.Load(),
		"download":    t.Download.Load(),
		"start":       t.CreatedAt,
		"chains":      t.Chain,
		"rule":        rule,
		"rulePayload": "",
	})
}

type Tracker interface {
	Metadata() TrackerMetadata
	Close() error
}

type TCPConn struct {
	N.ExtendedConn
	metadata TrackerMetadata
	manager  *Manager
}

func (tt *TCPConn) Metadata() TrackerMetadata {
	return tt.metadata
}

func (tt *TCPConn) Close() error {
	tt.manager.Leave(tt)
	return tt.ExtendedConn.Close()
}

func (tt *TCPConn) Upstream() any {
	return tt.ExtendedConn
}

func (tt *TCPConn) ReaderReplaceable() bool {
	return true
}

func (tt *TCPConn) WriterReplaceable() bool {
	return true
}

func NewTCPTracker(conn net.Conn, manager *Manager, metadata adapter.InboundContext, outboundManager adapter.OutboundManager, matchRule adapter.Rule, matchOutbound adapter.Outbound) *TCPConn {
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
		next = outboundManager.Default().Tag()
	}
	for {
		detour, loaded := outboundManager.Outbound(next)
		if !loaded {
			break
		}
		chain = append(chain, next)
		outbound = detour.Tag()
		outboundType = detour.Type()
		group, isGroup := detour.(adapter.OutboundGroup)
		if !isGroup {
			break
		}
		next = group.Now()
	}
	upload := new(atomic.Int64)
	download := new(atomic.Int64)
	tracker := &TCPConn{
		ExtendedConn: bufio.NewCounterConn(conn, []N.CountFunc{func(n int64) {
			upload.Add(n)
			manager.PushUploaded(n)
		}}, []N.CountFunc{func(n int64) {
			download.Add(n)
			manager.PushDownloaded(n)
		}}),
		metadata: TrackerMetadata{
			ID:           id,
			Metadata:     metadata,
			CreatedAt:    time.Now(),
			Upload:       upload,
			Download:     download,
			Chain:        common.Reverse(chain),
			Rule:         matchRule,
			Outbound:     outbound,
			OutboundType: outboundType,
		},
		manager: manager,
	}
	manager.Join(tracker)
	return tracker
}

type UDPConn struct {
	N.PacketConn `json:"-"`
	metadata     TrackerMetadata
	manager      *Manager
}

func (ut *UDPConn) Metadata() TrackerMetadata {
	return ut.metadata
}

func (ut *UDPConn) Close() error {
	ut.manager.Leave(ut)
	return ut.PacketConn.Close()
}

func (ut *UDPConn) Upstream() any {
	return ut.PacketConn
}

func (ut *UDPConn) ReaderReplaceable() bool {
	return true
}

func (ut *UDPConn) WriterReplaceable() bool {
	return true
}

func NewUDPTracker(conn N.PacketConn, manager *Manager, metadata adapter.InboundContext, outboundManager adapter.OutboundManager, matchRule adapter.Rule, matchOutbound adapter.Outbound) *UDPConn {
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
		next = outboundManager.Default().Tag()
	}
	for {
		detour, loaded := outboundManager.Outbound(next)
		if !loaded {
			break
		}
		chain = append(chain, next)
		outbound = detour.Tag()
		outboundType = detour.Type()
		group, isGroup := detour.(adapter.OutboundGroup)
		if !isGroup {
			break
		}
		next = group.Now()
	}
	upload := new(atomic.Int64)
	download := new(atomic.Int64)
	trackerConn := &UDPConn{
		PacketConn: bufio.NewCounterPacketConn(conn, []N.CountFunc{func(n int64) {
			upload.Add(n)
			manager.PushUploaded(n)
		}}, []N.CountFunc{func(n int64) {
			download.Add(n)
			manager.PushDownloaded(n)
		}}),
		metadata: TrackerMetadata{
			ID:           id,
			Metadata:     metadata,
			CreatedAt:    time.Now(),
			Upload:       upload,
			Download:     download,
			Chain:        common.Reverse(chain),
			Rule:         matchRule,
			Outbound:     outbound,
			OutboundType: outboundType,
		},
		manager: manager,
	}
	manager.Join(trackerConn)
	return trackerConn
}
