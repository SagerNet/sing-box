package adapter

import (
	"net"

	N "github.com/sagernet/sing/common/network"
)

type ManagedSSMServer interface {
	Inbound
	SetTracker(tracker SSMTracker)
	UpdateUsers(users []string, uPSKs []string) error
}

type SSMTracker interface {
	TrackConnection(conn net.Conn, metadata InboundContext) net.Conn
	TrackPacketConnection(conn N.PacketConn, metadata InboundContext) N.PacketConn
}
