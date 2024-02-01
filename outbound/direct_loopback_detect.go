package outbound

import (
	"net"
	"net/netip"
	"sync"

	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type loopBackDetector struct {
	connAccess       sync.RWMutex
	packetConnAccess sync.RWMutex
	connMap          map[netip.AddrPort]bool
	packetConnMap    map[netip.AddrPort]bool
}

func newLoopBackDetector() *loopBackDetector {
	return &loopBackDetector{
		connMap:       make(map[netip.AddrPort]bool),
		packetConnMap: make(map[netip.AddrPort]bool),
	}
}

func (l *loopBackDetector) NewConn(conn net.Conn) net.Conn {
	connAddr := M.AddrPortFromNet(conn.LocalAddr())
	if !connAddr.IsValid() {
		return conn
	}
	if udpConn, isUDPConn := conn.(abstractUDPConn); isUDPConn {
		l.packetConnAccess.Lock()
		l.packetConnMap[connAddr] = true
		l.packetConnAccess.Unlock()
		return &loopBackDetectUDPWrapper{abstractUDPConn: udpConn, detector: l, connAddr: connAddr}
	} else {
		l.connAccess.Lock()
		l.connMap[connAddr] = true
		l.connAccess.Unlock()
		return &loopBackDetectWrapper{Conn: conn, detector: l, connAddr: connAddr}
	}
}

func (l *loopBackDetector) NewPacketConn(conn N.NetPacketConn) N.NetPacketConn {
	connAddr := M.AddrPortFromNet(conn.LocalAddr())
	if !connAddr.IsValid() {
		return conn
	}
	l.packetConnAccess.Lock()
	l.packetConnMap[connAddr] = true
	l.packetConnAccess.Unlock()
	return &loopBackDetectPacketWrapper{NetPacketConn: conn, detector: l, connAddr: connAddr}
}

func (l *loopBackDetector) CheckConn(connAddr netip.AddrPort) bool {
	l.connAccess.RLock()
	defer l.connAccess.RUnlock()
	return l.connMap[connAddr]
}

func (l *loopBackDetector) CheckPacketConn(connAddr netip.AddrPort) bool {
	l.packetConnAccess.RLock()
	defer l.packetConnAccess.RUnlock()
	return l.packetConnMap[connAddr]
}

type loopBackDetectWrapper struct {
	net.Conn
	detector  *loopBackDetector
	connAddr  netip.AddrPort
	closeOnce sync.Once
}

func (w *loopBackDetectWrapper) Close() error {
	w.closeOnce.Do(func() {
		w.detector.connAccess.Lock()
		delete(w.detector.connMap, w.connAddr)
		w.detector.connAccess.Unlock()
	})
	return w.Conn.Close()
}

func (w *loopBackDetectWrapper) ReaderReplaceable() bool {
	return true
}

func (w *loopBackDetectWrapper) WriterReplaceable() bool {
	return true
}

func (w *loopBackDetectWrapper) Upstream() any {
	return w.Conn
}

type loopBackDetectPacketWrapper struct {
	N.NetPacketConn
	detector  *loopBackDetector
	connAddr  netip.AddrPort
	closeOnce sync.Once
}

func (w *loopBackDetectPacketWrapper) Close() error {
	w.closeOnce.Do(func() {
		w.detector.packetConnAccess.Lock()
		delete(w.detector.packetConnMap, w.connAddr)
		w.detector.packetConnAccess.Unlock()
	})
	return w.NetPacketConn.Close()
}

func (w *loopBackDetectPacketWrapper) ReaderReplaceable() bool {
	return true
}

func (w *loopBackDetectPacketWrapper) WriterReplaceable() bool {
	return true
}

func (w *loopBackDetectPacketWrapper) Upstream() any {
	return w.NetPacketConn
}

type abstractUDPConn interface {
	net.Conn
	net.PacketConn
}

type loopBackDetectUDPWrapper struct {
	abstractUDPConn
	detector  *loopBackDetector
	connAddr  netip.AddrPort
	closeOnce sync.Once
}

func (w *loopBackDetectUDPWrapper) Close() error {
	w.closeOnce.Do(func() {
		w.detector.packetConnAccess.Lock()
		delete(w.detector.packetConnMap, w.connAddr)
		w.detector.packetConnAccess.Unlock()
	})
	return w.abstractUDPConn.Close()
}

func (w *loopBackDetectUDPWrapper) ReaderReplaceable() bool {
	return true
}

func (w *loopBackDetectUDPWrapper) WriterReplaceable() bool {
	return true
}

func (w *loopBackDetectUDPWrapper) Upstream() any {
	return w.abstractUDPConn
}
