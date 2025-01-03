package conntrack

import (
	"net"
	"net/netip"
	"time"

	N "github.com/sagernet/sing/common/network"
)

// TODO: add to N
type AbstractPacketConn interface {
	Close() error
	LocalAddr() net.Addr
	SetDeadline(t time.Time) error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
}

type Tracker interface {
	NewConn(conn net.Conn) (net.Conn, error)
	NewPacketConn(conn net.PacketConn) (net.PacketConn, error)
	NewConnEx(conn net.Conn) (N.CloseHandlerFunc, error)
	NewPacketConnEx(conn AbstractPacketConn) (N.CloseHandlerFunc, error)
	CheckConn(source netip.AddrPort, destination netip.AddrPort) bool
	CheckPacketConn(source netip.AddrPort) bool
	AddPendingDestination(destination netip.AddrPort) func()
	CheckDestination(destination netip.AddrPort) bool
	KillerCheck() error
	Count() int
	Close()
}
