package route

import (
	"net"
	"net/netip"
	"os"
	"sync"

	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type fakeIPNATPacketConn struct {
	N.NetPacketConn
	origin             M.Socksaddr
	destination        M.Socksaddr
	directAccess       sync.RWMutex
	directDestinations map[netip.Addr]struct{}
}

func newFakeIPNATPacketConn(conn N.NetPacketConn, origin M.Socksaddr, destination M.Socksaddr) *fakeIPNATPacketConn {
	return &fakeIPNATPacketConn{
		NetPacketConn:      conn,
		origin:             origin,
		destination:        destination,
		directDestinations: make(map[netip.Addr]struct{}),
	}
}

func (c *fakeIPNATPacketConn) rewriteReadDestination(destination M.Socksaddr) M.Socksaddr {
	if destination.Addr == c.origin.Addr {
		return M.Socksaddr{
			Addr: c.destination.Addr,
			Fqdn: c.destination.Fqdn,
			Port: destination.Port,
		}
	}
	if destination.Addr.IsValid() {
		c.directAccess.RLock()
		_, recorded := c.directDestinations[destination.Addr]
		c.directAccess.RUnlock()
		if !recorded {
			c.directAccess.Lock()
			c.directDestinations[destination.Addr] = struct{}{}
			c.directAccess.Unlock()
		}
	}
	return destination
}

func (c *fakeIPNATPacketConn) rewriteWriteDestination(destination M.Socksaddr) M.Socksaddr {
	if destination.Addr.IsValid() {
		c.directAccess.RLock()
		_, direct := c.directDestinations[destination.Addr]
		c.directAccess.RUnlock()
		if direct {
			return destination
		}
	}
	return M.Socksaddr{
		Addr: c.origin.Addr,
		Port: destination.Port,
	}
}

func (c *fakeIPNATPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	n, addr, err = c.NetPacketConn.ReadFrom(p)
	if err != nil {
		return
	}
	addr = c.rewriteReadDestination(M.SocksaddrFromNet(addr)).UDPAddr()
	return
}

func (c *fakeIPNATPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	return c.NetPacketConn.WriteTo(p, c.rewriteWriteDestination(M.SocksaddrFromNet(addr)).UDPAddr())
}

func (c *fakeIPNATPacketConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	destination, err = c.NetPacketConn.ReadPacket(buffer)
	if err != nil {
		return
	}
	destination = c.rewriteReadDestination(destination)
	return
}

func (c *fakeIPNATPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	return c.NetPacketConn.WritePacket(buffer, c.rewriteWriteDestination(destination))
}

func (c *fakeIPNATPacketConn) CreateReadWaiter() (N.PacketReadWaiter, bool) {
	waiter, created := bufio.CreatePacketReadWaiter(c.NetPacketConn)
	if !created {
		return nil, false
	}
	return &waitFakeIPNATPacketConn{c, waiter}, true
}

func (c *fakeIPNATPacketConn) CreatePacketBatchReadWaiter() (N.PacketBatchReadWaiter, bool) {
	waiter, created := bufio.CreatePacketBatchReadWaiter(c.NetPacketConn)
	if !created {
		return nil, false
	}
	return &batchWaitFakeIPNATPacketConn{c, waiter}, true
}

func (c *fakeIPNATPacketConn) CreateConnectedPacketBatchReadWaiter() (N.ConnectedPacketBatchReadWaiter, bool) {
	waiter, created := bufio.CreateConnectedPacketBatchReadWaiter(c.NetPacketConn)
	if !created {
		return nil, false
	}
	return &connectedBatchWaitFakeIPNATPacketConn{c, waiter}, true
}

func (c *fakeIPNATPacketConn) CreatePacketBatchWriter() (N.PacketBatchWriter, bool) {
	writer, created := bufio.CreatePacketBatchWriter(c.NetPacketConn)
	if !created {
		return nil, false
	}
	return &fakeIPNATPacketBatchWriter{c, writer}, true
}

func (c *fakeIPNATPacketConn) CreateConnectedPacketBatchWriter() (N.ConnectedPacketBatchWriter, bool) {
	return bufio.CreateConnectedPacketBatchWriter(c.NetPacketConn)
}

func (c *fakeIPNATPacketConn) UpdateDestination(destinationAddress netip.Addr) {
	c.destination = M.SocksaddrFrom(destinationAddress, c.destination.Port)
}

func (c *fakeIPNATPacketConn) RemoteAddr() net.Addr {
	return c.destination.UDPAddr()
}

func (c *fakeIPNATPacketConn) Upstream() any {
	return c.NetPacketConn
}

type waitFakeIPNATPacketConn struct {
	*fakeIPNATPacketConn
	readWaiter N.PacketReadWaiter
}

func (c *waitFakeIPNATPacketConn) InitializeReadWaiter(options N.ReadWaitOptions) (needCopy bool) {
	return c.readWaiter.InitializeReadWaiter(options)
}

func (c *waitFakeIPNATPacketConn) WaitReadPacket() (buffer *buf.Buffer, destination M.Socksaddr, err error) {
	buffer, destination, err = c.readWaiter.WaitReadPacket()
	if err != nil {
		return
	}
	destination = c.rewriteReadDestination(destination)
	return
}

type batchWaitFakeIPNATPacketConn struct {
	*fakeIPNATPacketConn
	readWaiter N.PacketBatchReadWaiter
}

func (c *batchWaitFakeIPNATPacketConn) InitializeReadWaiter(options N.ReadWaitOptions) (needCopy bool) {
	return c.readWaiter.InitializeReadWaiter(options)
}

func (c *batchWaitFakeIPNATPacketConn) WaitReadPackets() (buffers []*buf.Buffer, destinations []M.Socksaddr, err error) {
	buffers, destinations, err = c.readWaiter.WaitReadPackets()
	if err != nil {
		return
	}
	for index, destination := range destinations {
		destinations[index] = c.rewriteReadDestination(destination)
	}
	return
}

type connectedBatchWaitFakeIPNATPacketConn struct {
	*fakeIPNATPacketConn
	readWaiter N.ConnectedPacketBatchReadWaiter
}

func (c *connectedBatchWaitFakeIPNATPacketConn) InitializeReadWaiter(options N.ReadWaitOptions) (needCopy bool) {
	return c.readWaiter.InitializeReadWaiter(options)
}

func (c *connectedBatchWaitFakeIPNATPacketConn) WaitReadConnectedPackets() (buffers []*buf.Buffer, destination M.Socksaddr, err error) {
	buffers, destination, err = c.readWaiter.WaitReadConnectedPackets()
	if err != nil {
		return
	}
	destination = c.rewriteReadDestination(destination)
	return
}

type fakeIPNATPacketBatchWriter struct {
	*fakeIPNATPacketConn
	writer N.PacketBatchWriter
}

func (w *fakeIPNATPacketBatchWriter) WritePacketBatch(buffers []*buf.Buffer, destinations []M.Socksaddr) error {
	if len(buffers) == 0 || len(buffers) != len(destinations) {
		buf.ReleaseMulti(buffers)
		return os.ErrInvalid
	}
	for index, destination := range destinations {
		destinations[index] = w.rewriteWriteDestination(destination)
	}
	return w.writer.WritePacketBatch(buffers, destinations)
}

var (
	_ bufio.NATPacketConn                   = (*fakeIPNATPacketConn)(nil)
	_ N.PacketReadWaitCreator               = (*fakeIPNATPacketConn)(nil)
	_ N.PacketBatchReadWaitCreator          = (*fakeIPNATPacketConn)(nil)
	_ N.ConnectedPacketBatchReadWaitCreator = (*fakeIPNATPacketConn)(nil)
	_ N.PacketBatchWriteCreator             = (*fakeIPNATPacketConn)(nil)
	_ N.ConnectedPacketBatchWriteCreator    = (*fakeIPNATPacketConn)(nil)
)
