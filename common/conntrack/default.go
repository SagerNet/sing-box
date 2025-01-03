package conntrack

import (
	"net"
	"net/netip"
	runtimeDebug "runtime/debug"
	"sync"
	"time"

	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/memory"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/x/list"
)

var _ Tracker = (*DefaultTracker)(nil)

type DefaultTracker struct {
	connAccess  sync.RWMutex
	connList    list.List[net.Conn]
	connAddress map[netip.AddrPort]netip.AddrPort

	packetConnAccess  sync.RWMutex
	packetConnList    list.List[AbstractPacketConn]
	packetConnAddress map[netip.AddrPort]bool

	pendingAccess sync.RWMutex
	pendingList   list.List[netip.AddrPort]

	killerEnabled   bool
	memoryLimit     uint64
	killerLastCheck time.Time
}

func NewDefaultTracker(killerEnabled bool, memoryLimit uint64) *DefaultTracker {
	return &DefaultTracker{
		connAddress:       make(map[netip.AddrPort]netip.AddrPort),
		packetConnAddress: make(map[netip.AddrPort]bool),
		killerEnabled:     killerEnabled,
		memoryLimit:       memoryLimit,
	}
}

func (t *DefaultTracker) NewConn(conn net.Conn) (net.Conn, error) {
	err := t.KillerCheck()
	if err != nil {
		conn.Close()
		return nil, err
	}
	t.connAccess.Lock()
	element := t.connList.PushBack(conn)
	t.connAddress[M.AddrPortFromNet(conn.LocalAddr())] = M.AddrPortFromNet(conn.RemoteAddr())
	t.connAccess.Unlock()
	return &Conn{
		Conn: conn,
		closeFunc: common.OnceFunc(func() {
			t.removeConn(element)
		}),
	}, nil
}

func (t *DefaultTracker) NewConnEx(conn net.Conn) (N.CloseHandlerFunc, error) {
	err := t.KillerCheck()
	if err != nil {
		conn.Close()
		return nil, err
	}
	t.connAccess.Lock()
	element := t.connList.PushBack(conn)
	t.connAddress[M.AddrPortFromNet(conn.LocalAddr())] = M.AddrPortFromNet(conn.RemoteAddr())
	t.connAccess.Unlock()
	return N.OnceClose(func(it error) {
		t.removeConn(element)
	}), nil
}

func (t *DefaultTracker) NewPacketConn(conn net.PacketConn) (net.PacketConn, error) {
	err := t.KillerCheck()
	if err != nil {
		conn.Close()
		return nil, err
	}
	t.packetConnAccess.Lock()
	element := t.packetConnList.PushBack(conn)
	t.packetConnAddress[M.AddrPortFromNet(conn.LocalAddr())] = true
	t.packetConnAccess.Unlock()
	return &PacketConn{
		PacketConn: conn,
		closeFunc: common.OnceFunc(func() {
			t.removePacketConn(element)
		}),
	}, nil
}

func (t *DefaultTracker) NewPacketConnEx(conn AbstractPacketConn) (N.CloseHandlerFunc, error) {
	err := t.KillerCheck()
	if err != nil {
		conn.Close()
		return nil, err
	}
	t.packetConnAccess.Lock()
	element := t.packetConnList.PushBack(conn)
	t.packetConnAddress[M.AddrPortFromNet(conn.LocalAddr())] = true
	t.packetConnAccess.Unlock()
	return N.OnceClose(func(it error) {
		t.removePacketConn(element)
	}), nil
}

func (t *DefaultTracker) CheckConn(source netip.AddrPort, destination netip.AddrPort) bool {
	t.connAccess.RLock()
	defer t.connAccess.RUnlock()
	return t.connAddress[source] == destination
}

func (t *DefaultTracker) CheckPacketConn(source netip.AddrPort) bool {
	t.packetConnAccess.RLock()
	defer t.packetConnAccess.RUnlock()
	return t.packetConnAddress[source]
}

func (t *DefaultTracker) AddPendingDestination(destination netip.AddrPort) func() {
	t.pendingAccess.Lock()
	defer t.pendingAccess.Unlock()
	element := t.pendingList.PushBack(destination)
	return func() {
		t.pendingAccess.Lock()
		defer t.pendingAccess.Unlock()
		t.pendingList.Remove(element)
	}
}

func (t *DefaultTracker) CheckDestination(destination netip.AddrPort) bool {
	t.pendingAccess.RLock()
	defer t.pendingAccess.RUnlock()
	for element := t.pendingList.Front(); element != nil; element = element.Next() {
		if element.Value == destination {
			return true
		}
	}
	return false
}

func (t *DefaultTracker) KillerCheck() error {
	if !t.killerEnabled {
		return nil
	}
	nowTime := time.Now()
	if nowTime.Sub(t.killerLastCheck) < 3*time.Second {
		return nil
	}
	t.killerLastCheck = nowTime
	if memory.Total() > t.memoryLimit {
		t.Close()
		go func() {
			time.Sleep(time.Second)
			runtimeDebug.FreeOSMemory()
		}()
		return E.New("out of memory")
	}
	return nil
}

func (t *DefaultTracker) Count() int {
	t.connAccess.RLock()
	defer t.connAccess.RUnlock()
	t.packetConnAccess.RLock()
	defer t.packetConnAccess.RUnlock()
	return t.connList.Len() + t.packetConnList.Len()
}

func (t *DefaultTracker) Close() {
	t.connAccess.Lock()
	for element := t.connList.Front(); element != nil; element = element.Next() {
		element.Value.Close()
	}
	t.connList.Init()
	t.connAccess.Unlock()
	t.packetConnAccess.Lock()
	for element := t.packetConnList.Front(); element != nil; element = element.Next() {
		element.Value.Close()
	}
	t.packetConnList.Init()
	t.packetConnAccess.Unlock()
}

func (t *DefaultTracker) removeConn(element *list.Element[net.Conn]) {
	t.connAccess.Lock()
	defer t.connAccess.Unlock()
	delete(t.connAddress, M.AddrPortFromNet(element.Value.LocalAddr()))
	t.connList.Remove(element)
}

func (t *DefaultTracker) removePacketConn(element *list.Element[AbstractPacketConn]) {
	t.packetConnAccess.Lock()
	defer t.packetConnAccess.Unlock()
	delete(t.packetConnAddress, M.AddrPortFromNet(element.Value.LocalAddr()))
	t.packetConnList.Remove(element)
}

type Conn struct {
	net.Conn
	closeFunc func()
}

func (c *Conn) Close() error {
	c.closeFunc()
	return c.Conn.Close()
}

func (c *Conn) Upstream() any {
	return c.Conn
}

func (c *Conn) ReaderReplaceable() bool {
	return true
}

func (c *Conn) WriterReplaceable() bool {
	return true
}

type PacketConn struct {
	net.PacketConn
	closeFunc func()
}

func (c *PacketConn) Close() error {
	c.closeFunc()
	return c.PacketConn.Close()
}

func (c *PacketConn) Upstream() any {
	return c.PacketConn
}

func (c *PacketConn) ReaderReplaceable() bool {
	return true
}

func (c *PacketConn) WriterReplaceable() bool {
	return true
}
