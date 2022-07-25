package trafficontrol

import (
	"net"
	"net/netip"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/gofrs/uuid"
	"go.uber.org/atomic"
)

type Metadata struct {
	NetWork     string     `json:"network"`
	Type        string     `json:"type"`
	SrcIP       netip.Addr `json:"sourceIP"`
	DstIP       netip.Addr `json:"destinationIP"`
	SrcPort     string     `json:"sourcePort"`
	DstPort     string     `json:"destinationPort"`
	Host        string     `json:"host"`
	DNSMode     string     `json:"dnsMode"`
	ProcessPath string     `json:"processPath"`
}

type tracker interface {
	ID() string
	Close() error
	Leave()
}

type trackerInfo struct {
	UUID          uuid.UUID     `json:"id"`
	Metadata      Metadata      `json:"metadata"`
	UploadTotal   *atomic.Int64 `json:"upload"`
	DownloadTotal *atomic.Int64 `json:"download"`
	Start         time.Time     `json:"start"`
	Chain         []string      `json:"chains"`
	Rule          string        `json:"rule"`
	RulePayload   string        `json:"rulePayload"`
}

type tcpTracker struct {
	net.Conn `json:"-"`
	*trackerInfo
	manager *Manager
}

func (tt *tcpTracker) ID() string {
	return tt.UUID.String()
}

func (tt *tcpTracker) Read(b []byte) (int, error) {
	n, err := tt.Conn.Read(b)
	upload := int64(n)
	tt.manager.PushUploaded(upload)
	tt.UploadTotal.Add(upload)
	return n, err
}

func (tt *tcpTracker) Write(b []byte) (int, error) {
	n, err := tt.Conn.Write(b)
	download := int64(n)
	tt.manager.PushDownloaded(download)
	tt.DownloadTotal.Add(download)
	return n, err
}

func (tt *tcpTracker) Close() error {
	tt.manager.Leave(tt)
	return tt.Conn.Close()
}

func (tt *tcpTracker) Leave() {
	tt.manager.Leave(tt)
}

func NewTCPTracker(conn net.Conn, manager *Manager, metadata Metadata, router adapter.Router, rule adapter.Rule) *tcpTracker {
	uuid, _ := uuid.NewV4()

	var chain []string
	var next string
	if rule == nil {
		next = router.DefaultOutbound(C.NetworkTCP).Tag()
	} else {
		next = rule.Outbound()
	}
	for {
		chain = append(chain, next)
		detour, loaded := router.Outbound(next)
		if !loaded {
			break
		}
		group, isGroup := detour.(adapter.OutboundGroup)
		if !isGroup {
			break
		}
		next = group.Now()
	}

	t := &tcpTracker{
		Conn:    conn,
		manager: manager,
		trackerInfo: &trackerInfo{
			UUID:          uuid,
			Start:         time.Now(),
			Metadata:      metadata,
			Chain:         common.Reverse(chain),
			Rule:          "",
			UploadTotal:   atomic.NewInt64(0),
			DownloadTotal: atomic.NewInt64(0),
		},
	}

	if rule != nil {
		t.trackerInfo.Rule = rule.String() + " => " + rule.Outbound()
	} else {
		t.trackerInfo.Rule = "final"
	}

	manager.Join(t)
	return t
}

type udpTracker struct {
	N.PacketConn `json:"-"`
	*trackerInfo
	manager *Manager
}

func (ut *udpTracker) ID() string {
	return ut.UUID.String()
}

func (ut *udpTracker) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	destination, err = ut.PacketConn.ReadPacket(buffer)
	if err == nil {
		upload := int64(buffer.Len())
		ut.manager.PushUploaded(upload)
		ut.UploadTotal.Add(upload)
	}
	return
}

func (ut *udpTracker) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	download := int64(buffer.Len())
	err := ut.PacketConn.WritePacket(buffer, destination)
	if err != nil {
		return err
	}
	ut.manager.PushDownloaded(download)
	ut.DownloadTotal.Add(download)
	return nil
}

func (ut *udpTracker) Close() error {
	ut.manager.Leave(ut)
	return ut.PacketConn.Close()
}

func (ut *udpTracker) Leave() {
	ut.manager.Leave(ut)
}

func NewUDPTracker(conn N.PacketConn, manager *Manager, metadata Metadata, router adapter.Router, rule adapter.Rule) *udpTracker {
	uuid, _ := uuid.NewV4()

	var chain []string
	var next string
	if rule == nil {
		next = router.DefaultOutbound(C.NetworkUDP).Tag()
	} else {
		next = rule.Outbound()
	}
	for {
		chain = append(chain, next)
		detour, loaded := router.Outbound(next)
		if !loaded {
			break
		}
		group, isGroup := detour.(adapter.OutboundGroup)
		if !isGroup {
			break
		}
		next = group.Now()
	}

	ut := &udpTracker{
		PacketConn: conn,
		manager:    manager,
		trackerInfo: &trackerInfo{
			UUID:          uuid,
			Start:         time.Now(),
			Metadata:      metadata,
			Chain:         common.Reverse(chain),
			Rule:          "",
			UploadTotal:   atomic.NewInt64(0),
			DownloadTotal: atomic.NewInt64(0),
		},
	}

	if rule != nil {
		ut.trackerInfo.Rule = rule.String() + " => " + rule.Outbound()
	} else {
		ut.trackerInfo.Rule = "final"
	}

	manager.Join(ut)
	return ut
}
