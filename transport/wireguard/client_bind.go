package wireguard

import (
	"context"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/wireguard-go/conn"
)

var _ conn.Bind = (*ClientBind)(nil)

type ClientBind struct {
	ctx                 context.Context
	errorHandler        E.Handler
	dialer              N.Dialer
	reservedForEndpoint map[netip.AddrPort][3]uint8
	connAccess          sync.Mutex
	conn                *wireConn
	done                chan struct{}
	isConnect           bool
	connectAddr         netip.AddrPort
	reserved            [3]uint8
}

func NewClientBind(ctx context.Context, errorHandler E.Handler, dialer N.Dialer, isConnect bool, connectAddr netip.AddrPort, reserved [3]uint8) *ClientBind {
	return &ClientBind{
		ctx:                 ctx,
		errorHandler:        errorHandler,
		dialer:              dialer,
		reservedForEndpoint: make(map[netip.AddrPort][3]uint8),
		isConnect:           isConnect,
		connectAddr:         connectAddr,
		reserved:            reserved,
	}
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
		udpConn, err := c.dialer.DialContext(c.ctx, N.NetworkUDP, M.SocksaddrFromNetIP(c.connectAddr))
		if err != nil {
			return nil, err
		}
		c.conn = &wireConn{
			PacketConn: bufio.NewUnbindPacketConn(udpConn),
			done:       make(chan struct{}),
		}
	} else {
		udpConn, err := c.dialer.ListenPacket(c.ctx, M.Socksaddr{Addr: netip.IPv4Unspecified()})
		if err != nil {
			return nil, err
		}
		c.conn = &wireConn{
			PacketConn: bufio.NewPacketConn(udpConn),
			done:       make(chan struct{}),
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

func (c *ClientBind) receive(packets [][]byte, sizes []int, eps []conn.Endpoint) (count int, err error) {
	udpConn, err := c.connect()
	if err != nil {
		select {
		case <-c.done:
			return
		default:
		}
		c.errorHandler.NewError(context.Background(), E.Cause(err, "connect to server"))
		err = nil
		time.Sleep(time.Second)
		return
	}
	n, addr, err := udpConn.ReadFrom(packets[0])
	if err != nil {
		udpConn.Close()
		select {
		case <-c.done:
		default:
			c.errorHandler.NewError(context.Background(), E.Cause(err, "read packet"))
			err = nil
		}
		return
	}
	sizes[0] = n
	if n > 3 {
		b := packets[0]
		common.ClearArray(b[1:4])
	}
	eps[0] = Endpoint(M.AddrPortFromNet(addr))
	count = 1
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
	default:
		close(c.done)
	}
	return nil
}

func (c *ClientBind) SetMark(mark uint32) error {
	return nil
}

func (c *ClientBind) Send(bufs [][]byte, ep conn.Endpoint) error {
	udpConn, err := c.connect()
	if err != nil {
		return err
	}
	destination := netip.AddrPort(ep.(Endpoint))
	for _, b := range bufs {
		if len(b) > 3 {
			reserved, loaded := c.reservedForEndpoint[destination]
			if !loaded {
				reserved = c.reserved
			}
			copy(b[1:4], reserved[:])
		}
		_, err = udpConn.WriteTo(b, M.SocksaddrFromNetIP(destination))
		if err != nil {
			udpConn.Close()
			return err
		}
	}
	return nil
}

func (c *ClientBind) ParseEndpoint(s string) (conn.Endpoint, error) {
	ap, err := netip.ParseAddrPort(s)
	if err != nil {
		return nil, err
	}
	return Endpoint(ap), nil
}

func (c *ClientBind) BatchSize() int {
	return 1
}

func (c *ClientBind) SetReservedForEndpoint(destination netip.AddrPort, reserved [3]byte) {
	c.reservedForEndpoint[destination] = reserved
}

type wireConn struct {
	net.PacketConn
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
	w.PacketConn.Close()
	close(w.done)
	return nil
}
