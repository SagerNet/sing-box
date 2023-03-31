package wireguard

import (
	"context"
	"net"
	"net/netip"
	"sync"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/wireguard-go/conn"
)

var _ conn.Bind = (*ClientBind)(nil)

type ClientBind struct {
	ctx                 context.Context
	dialer              N.Dialer
	reservedForEndpoint map[M.Socksaddr][3]uint8
	connAccess          sync.Mutex
	conn                *wireConn
	done                chan struct{}
	isConnect           bool
	connectAddr         M.Socksaddr
	reserved            [3]uint8
}

func NewClientBind(ctx context.Context, dialer N.Dialer, isConnect bool, connectAddr M.Socksaddr, reserved [3]uint8) *ClientBind {
	return &ClientBind{
		ctx:                 ctx,
		dialer:              dialer,
		reservedForEndpoint: make(map[M.Socksaddr][3]uint8),
		isConnect:           isConnect,
		connectAddr:         connectAddr,
		reserved:            reserved,
	}
}

func (c *ClientBind) SetReservedForEndpoint(destination M.Socksaddr, reserved [3]byte) {
	c.reservedForEndpoint[destination] = reserved
}

func (c *ClientBind) connect() (*wireConn, error) {
	serverConn := c.conn
	if serverConn != nil {
		select {
		case <-serverConn.done:
			serverConn = nil
		default:
			return serverConn, nil
		}
	}
	c.connAccess.Lock()
	defer c.connAccess.Unlock()
	serverConn = c.conn
	if serverConn != nil {
		select {
		case <-serverConn.done:
			serverConn = nil
		default:
			return serverConn, nil
		}
	}
	if c.isConnect {
		udpConn, err := c.dialer.DialContext(c.ctx, N.NetworkUDP, c.connectAddr)
		if err != nil {
			return nil, &wireError{err}
		}
		c.conn = &wireConn{
			NetPacketConn: &bufio.UnbindPacketConn{
				ExtendedConn: bufio.NewExtendedConn(udpConn),
				Addr:         c.connectAddr,
			},
			done: make(chan struct{}),
		}
	} else {
		udpConn, err := c.dialer.ListenPacket(c.ctx, M.Socksaddr{Addr: netip.IPv4Unspecified()})
		if err != nil {
			return nil, &wireError{err}
		}
		c.conn = &wireConn{
			NetPacketConn: bufio.NewPacketConn(udpConn),
			done:          make(chan struct{}),
		}
	}
	return c.conn, nil
}

func (c *ClientBind) Open(port uint16) (fns []conn.ReceiveFunc, actualPort uint16, err error) {
	select {
	case <-c.done:
		err = net.ErrClosed
		return
	default:
	}
	return []conn.ReceiveFunc{c.receive}, 0, nil
}

func (c *ClientBind) receive(b []byte) (n int, ep conn.Endpoint, err error) {
	udpConn, err := c.connect()
	if err != nil {
		err = &wireError{err}
		return
	}
	buffer := buf.With(b)
	destination, err := udpConn.ReadPacket(buffer)
	if err != nil {
		udpConn.Close()
		select {
		case <-c.done:
		default:
			err = &wireError{err}
		}
		return
	}
	n = buffer.Len()
	if buffer.Start() > 0 {
		copy(b, buffer.Bytes())
	}
	if n > 3 {
		b[1] = 0
		b[2] = 0
		b[3] = 0
	}
	ep = Endpoint(destination)
	return
}

func (c *ClientBind) Reset() {
	common.Close(common.PtrOrNil(c.conn))
}

func (c *ClientBind) Close() error {
	common.Close(common.PtrOrNil(c.conn))
	if c.done == nil {
		c.done = make(chan struct{})
		return nil
	}
	select {
	case <-c.done:
		return net.ErrClosed
	default:
		close(c.done)
	}
	return nil
}

func (c *ClientBind) SetMark(mark uint32) error {
	return nil
}

func (c *ClientBind) Send(b []byte, ep conn.Endpoint) error {
	udpConn, err := c.connect()
	if err != nil {
		return err
	}
	destination := M.Socksaddr(ep.(Endpoint))
	if len(b) > 3 {
		reserved, loaded := c.reservedForEndpoint[destination]
		if !loaded {
			reserved = c.reserved
		}
		b[1] = reserved[0]
		b[2] = reserved[1]
		b[3] = reserved[2]
	}
	err = udpConn.WritePacket(buf.As(b), destination)
	if err != nil {
		udpConn.Close()
	}
	return err
}

func (c *ClientBind) ParseEndpoint(s string) (conn.Endpoint, error) {
	return Endpoint(M.ParseSocksaddr(s)), nil
}

type wireConn struct {
	N.NetPacketConn
	access sync.Mutex
	done   chan struct{}
}

func (w *wireConn) Close() error {
	w.access.Lock()
	defer w.access.Unlock()
	select {
	case <-w.done:
		return net.ErrClosed
	default:
	}
	w.NetPacketConn.Close()
	close(w.done)
	return nil
}
