package direct

import (
	"net"
	"net/netip"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type loopBackDetector struct {
	networkManager   adapter.NetworkManager
	connAccess       sync.RWMutex
	packetConnAccess sync.RWMutex
	connMap          map[netip.AddrPort]netip.AddrPort
	packetConnMap    map[uint16]uint16
}

func newLoopBackDetector(networkManager adapter.NetworkManager) *loopBackDetector {
	return &loopBackDetector{
		networkManager: networkManager,
		connMap:        make(map[netip.AddrPort]netip.AddrPort),
		packetConnMap:  make(map[uint16]uint16),
	}
}

func (l *loopBackDetector) NewConn(conn net.Conn) net.Conn {
	source := M.AddrPortFromNet(conn.LocalAddr())
	if !source.IsValid() {
		return conn
	}
	if udpConn, isUDPConn := conn.(abstractUDPConn); isUDPConn {
		if !source.Addr().IsLoopback() {
			_, err := l.networkManager.InterfaceFinder().ByAddr(source.Addr())
			if err != nil {
				return conn
			}
		}
		if !N.IsPublicAddr(source.Addr()) {
			return conn
		}
		l.packetConnAccess.Lock()
		l.packetConnMap[source.Port()] = M.AddrPortFromNet(conn.RemoteAddr()).Port()
		l.packetConnAccess.Unlock()
		return &loopBackDetectUDPWrapper{abstractUDPConn: udpConn, detector: l, connPort: source.Port()}
	} else {
		l.connAccess.Lock()
		l.connMap[source] = M.AddrPortFromNet(conn.RemoteAddr())
		l.connAccess.Unlock()
		return &loopBackDetectWrapper{Conn: conn, detector: l, connAddr: source}
	}
}

func (l *loopBackDetector) NewPacketConn(conn N.NetPacketConn, destination M.Socksaddr) N.NetPacketConn {
	source := M.AddrPortFromNet(conn.LocalAddr())
	if !source.IsValid() {
		return conn
	}
	if !source.Addr().IsLoopback() {
		_, err := l.networkManager.InterfaceFinder().ByAddr(source.Addr())
		if err != nil {
			return conn
		}
	}
	l.packetConnAccess.Lock()
	l.packetConnMap[source.Port()] = destination.AddrPort().Port()
	l.packetConnAccess.Unlock()
	return &loopBackDetectPacketWrapper{NetPacketConn: conn, detector: l, connPort: source.Port()}
}

func (l *loopBackDetector) CheckConn(source netip.AddrPort, local netip.AddrPort) bool {
	l.connAccess.RLock()
	defer l.connAccess.RUnlock()
	destination, loaded := l.connMap[source]
	return loaded && destination != local
}

func (l *loopBackDetector) CheckPacketConn(source netip.AddrPort, local netip.AddrPort) bool {
	if !source.IsValid() {
		return false
	}
	if !source.Addr().IsLoopback() {
		_, err := l.networkManager.InterfaceFinder().ByAddr(source.Addr())
		if err != nil {
			return false
		}
	}
	if N.IsPublicAddr(source.Addr()) {
		return false
	}
	l.packetConnAccess.RLock()
	defer l.packetConnAccess.RUnlock()
	destinationPort, loaded := l.packetConnMap[source.Port()]
	return loaded && destinationPort != local.Port()
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
	connPort  uint16
	closeOnce sync.Once
}

func (w *loopBackDetectPacketWrapper) Close() error {
	w.closeOnce.Do(func() {
		w.detector.packetConnAccess.Lock()
		delete(w.detector.packetConnMap, w.connPort)
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
	connPort  uint16
	closeOnce sync.Once
}

func (w *loopBackDetectUDPWrapper) Close() error {
	w.closeOnce.Do(func() {
		w.detector.packetConnAccess.Lock()
		delete(w.detector.packetConnMap, w.connPort)
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
