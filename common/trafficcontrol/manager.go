package trafficcontrol

import (
	"io"
	"net"
	"sync"
	"sync/atomic"

	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type Manager[U comparable] struct {
	access sync.Mutex
	users  map[U]*Traffic
}

type Traffic struct {
	Upload   uint64
	Download uint64
}

func NewManager[U comparable]() *Manager[U] {
	return &Manager[U]{
		users: make(map[U]*Traffic),
	}
}

func (m *Manager[U]) Reset() {
	m.users = make(map[U]*Traffic)
}

func (m *Manager[U]) TrackConnection(user U, conn net.Conn) net.Conn {
	m.access.Lock()
	defer m.access.Unlock()
	var traffic *Traffic
	if t, loaded := m.users[user]; loaded {
		traffic = t
	} else {
		traffic = new(Traffic)
		m.users[user] = traffic
	}
	return &TrackConn{conn, traffic}
}

func (m *Manager[U]) TrackPacketConnection(user U, conn N.PacketConn) N.PacketConn {
	m.access.Lock()
	defer m.access.Unlock()
	var traffic *Traffic
	if t, loaded := m.users[user]; loaded {
		traffic = t
	} else {
		traffic = new(Traffic)
		m.users[user] = traffic
	}
	return &TrackPacketConn{conn, traffic}
}

func (m *Manager[U]) ReadTraffics() map[U]Traffic {
	m.access.Lock()
	defer m.access.Unlock()

	trafficMap := make(map[U]Traffic)
	for user, traffic := range m.users {
		upload := atomic.SwapUint64(&traffic.Upload, 0)
		download := atomic.SwapUint64(&traffic.Download, 0)
		if upload == 0 && download == 0 {
			continue
		}
		trafficMap[user] = Traffic{
			Upload:   upload,
			Download: download,
		}
	}
	return trafficMap
}

type TrackConn struct {
	net.Conn
	*Traffic
}

func (c *TrackConn) Read(p []byte) (n int, err error) {
	n, err = c.Conn.Read(p)
	if n > 0 {
		atomic.AddUint64(&c.Upload, uint64(n))
	}
	return
}

func (c *TrackConn) Write(p []byte) (n int, err error) {
	n, err = c.Conn.Write(p)
	if n > 0 {
		atomic.AddUint64(&c.Download, uint64(n))
	}
	return
}

func (c *TrackConn) WriteTo(w io.Writer) (n int64, err error) {
	n, err = bufio.Copy(w, c.Conn)
	if n > 0 {
		atomic.AddUint64(&c.Upload, uint64(n))
	}
	return
}

func (c *TrackConn) ReadFrom(r io.Reader) (n int64, err error) {
	n, err = bufio.Copy(c.Conn, r)
	if n > 0 {
		atomic.AddUint64(&c.Download, uint64(n))
	}
	return
}

func (c *TrackConn) Upstream() any {
	return c.Conn
}

type TrackPacketConn struct {
	N.PacketConn
	*Traffic
}

func (c *TrackPacketConn) ReadPacket(buffer *buf.Buffer) (M.Socksaddr, error) {
	destination, err := c.PacketConn.ReadPacket(buffer)
	if err == nil {
		atomic.AddUint64(&c.Upload, uint64(buffer.Len()))
	}
	return destination, err
}

func (c *TrackPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	n := buffer.Len()
	err := c.PacketConn.WritePacket(buffer, destination)
	if err == nil {
		atomic.AddUint64(&c.Download, uint64(n))
	}
	return err
}

func (c *TrackPacketConn) Upstream() any {
	return c.PacketConn
}
