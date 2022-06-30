package udpnat

import (
	"context"
	"io"
	"net"
	"net/netip"
	"os"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/cache"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type Handler interface {
	NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error
	E.Handler
}

type Service[K comparable] struct {
	nat     *cache.LruCache[K, *conn]
	handler Handler
}

func New[K comparable](maxAge int64, handler Handler) *Service[K] {
	return &Service[K]{
		nat: cache.New(
			cache.WithAge[K, *conn](maxAge),
			cache.WithUpdateAgeOnGet[K, *conn](),
			cache.WithEvict[K, *conn](func(key K, conn *conn) {
				conn.Close()
			}),
		),
		handler: handler,
	}
}

func (s *Service[T]) NewPacketDirect(ctx context.Context, key T, conn N.PacketConn, buffer *buf.Buffer, metadata adapter.InboundContext) {
	s.NewContextPacket(ctx, key, buffer, metadata, func(natConn N.PacketConn) (context.Context, N.PacketWriter) {
		return ctx, &DirectBackWriter{conn, natConn}
	})
}

type DirectBackWriter struct {
	Source N.PacketConn
	Nat    N.PacketConn
}

func (w *DirectBackWriter) WritePacket(buffer *buf.Buffer, addr M.Socksaddr) error {
	return w.Source.WritePacket(buffer, M.SocksaddrFromNet(w.Nat.LocalAddr()))
}

func (w *DirectBackWriter) Upstream() any {
	return w.Source
}

func (s *Service[T]) NewPacket(ctx context.Context, key T, buffer *buf.Buffer, metadata adapter.InboundContext, init func(natConn N.PacketConn) N.PacketWriter) {
	s.NewContextPacket(ctx, key, buffer, metadata, func(natConn N.PacketConn) (context.Context, N.PacketWriter) {
		return ctx, init(natConn)
	})
}

func (s *Service[T]) NewContextPacket(ctx context.Context, key T, buffer *buf.Buffer, metadata adapter.InboundContext, init func(natConn N.PacketConn) (context.Context, N.PacketWriter)) {
	var maxAge int64
	switch metadata.Destination.Port {
	case 443, 853:
		maxAge = 30
	case 53, 3478:
		maxAge = 10
	}
	c, loaded := s.nat.LoadOrStoreWithAge(key, maxAge, func() *conn {
		c := &conn{
			data:       make(chan packet, 64),
			localAddr:  metadata.Source,
			remoteAddr: metadata.Destination,
			fastClose:  metadata.Destination.Port == 53,
		}
		c.ctx, c.cancel = context.WithCancel(ctx)
		return c
	})
	if !loaded {
		ctx, c.source = init(c)
		go func() {
			err := s.handler.NewPacketConnection(ctx, c, metadata)
			if err != nil {
				s.handler.NewError(ctx, err)
			}
			c.Close()
			s.nat.Delete(key)
		}()
	} else {
		c.localAddr = metadata.Source
	}
	if common.Done(c.ctx) {
		s.nat.Delete(key)
		if !common.Done(ctx) {
			s.NewContextPacket(ctx, key, buffer, metadata, init)
		}
		return
	}
	c.data <- packet{
		data:        buffer,
		destination: metadata.Destination,
	}
}

type packet struct {
	data        *buf.Buffer
	destination M.Socksaddr
}

type conn struct {
	ctx          context.Context
	cancel       context.CancelFunc
	data         chan packet
	localAddr    netip.AddrPort
	remoteAddr   M.Socksaddr
	source       N.PacketWriter
	fastClose    bool
	readDeadline atomic.Value
}

func (c *conn) ReadPacketThreadSafe() (buffer *buf.Buffer, addr M.Socksaddr, err error) {
	var deadline <-chan time.Time
	if d, ok := c.readDeadline.Load().(time.Time); ok && !d.IsZero() {
		timer := time.NewTimer(time.Until(d))
		defer timer.Stop()
		deadline = timer.C
	}
	select {
	case p := <-c.data:
		return p.data, p.destination, nil
	case <-c.ctx.Done():
		return nil, M.Socksaddr{}, io.ErrClosedPipe
	case <-deadline:
		return nil, M.Socksaddr{}, os.ErrDeadlineExceeded
	}
}

func (c *conn) ReadPacket(buffer *buf.Buffer) (addr M.Socksaddr, err error) {
	var deadline <-chan time.Time
	if d, ok := c.readDeadline.Load().(time.Time); ok && !d.IsZero() {
		timer := time.NewTimer(time.Until(d))
		defer timer.Stop()
		deadline = timer.C
	}
	select {
	case p := <-c.data:
		_, err = buffer.ReadFrom(p.data)
		p.data.Release()
		return p.destination, err
	case <-c.ctx.Done():
		return M.Socksaddr{}, io.ErrClosedPipe
	case <-deadline:
		return M.Socksaddr{}, os.ErrDeadlineExceeded
	}
}

func (c *conn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	if c.fastClose {
		defer c.Close()
	}
	return c.source.WritePacket(buffer, destination)
}

func (c *conn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	select {
	case pkt := <-c.data:
		n = copy(p, pkt.data.Bytes())
		pkt.data.Release()
		addr = pkt.destination.UDPAddr()
		return n, addr, nil
	case <-c.ctx.Done():
		return 0, nil, io.ErrClosedPipe
	}
}

func (c *conn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	if c.fastClose {
		defer c.Close()
	}
	return len(p), c.source.WritePacket(buf.As(p).ToOwned(), M.SocksaddrFromNet(addr))
}

func (c *conn) Close() error {
	select {
	case <-c.ctx.Done():
		return os.ErrClosed
	default:
	}
	c.cancel()
	return nil
}

func (c *conn) LocalAddr() net.Addr {
	return M.SocksaddrFromNetIP(c.localAddr).UDPAddr()
}

func (c *conn) RemoteAddr() net.Addr {
	return c.remoteAddr.UDPAddr()
}

func (c *conn) SetDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *conn) SetReadDeadline(t time.Time) error {
	c.readDeadline.Store(t)
	return nil
}

func (c *conn) SetWriteDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *conn) Upstream() any {
	return c.source
}
