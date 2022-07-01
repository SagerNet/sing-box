package outbound

import (
	"context"
	"net"
	"runtime"
	"sync"
	"time"

	"github.com/database64128/tfo-go"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/config"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type myOutboundAdapter struct {
	protocol string
	logger   log.Logger
	tag      string
	dialer   N.Dialer
}

func (a *myOutboundAdapter) Type() string {
	return a.protocol
}

func (a *myOutboundAdapter) Tag() string {
	return a.tag
}

type defaultDialer struct {
	tfo.Dialer
	net.ListenConfig
}

func (d *defaultDialer) DialContext(ctx context.Context, network string, address M.Socksaddr) (net.Conn, error) {
	return d.Dialer.DialContext(ctx, network, address.String())
}

func (d *defaultDialer) ListenPacket(ctx context.Context) (net.PacketConn, error) {
	return d.ListenConfig.ListenPacket(ctx, "udp", "")
}

func newDialer(options config.DialerOptions) N.Dialer {
	var dialer net.Dialer
	var listener net.ListenConfig
	if options.BindInterface != "" {
		dialer.Control = control.Append(dialer.Control, control.BindToInterface(options.BindInterface))
		listener.Control = control.Append(listener.Control, control.BindToInterface(options.BindInterface))
	}
	if options.RoutingMark != 0 {
		dialer.Control = control.Append(dialer.Control, control.RoutingMark(options.RoutingMark))
		listener.Control = control.Append(listener.Control, control.RoutingMark(options.RoutingMark))
	}
	if options.ReuseAddr {
		listener.Control = control.Append(listener.Control, control.ReuseAddr())
	}
	if options.ConnectTimeout != 0 {
		dialer.Timeout = time.Duration(options.ConnectTimeout) * time.Second
	}
	return &defaultDialer{tfo.Dialer{Dialer: dialer, DisableTFO: !options.TCPFastOpen}, listener}
}

type lazyDialer struct {
	router   adapter.Router
	options  config.DialerOptions
	dialer   N.Dialer
	initOnce sync.Once
	initErr  error
}

func NewDialer(router adapter.Router, options config.DialerOptions) N.Dialer {
	if options.Detour == "" {
		return newDialer(options)
	}
	return &lazyDialer{
		router:  router,
		options: options,
	}
}

func (d *lazyDialer) Dialer() (N.Dialer, error) {
	d.initOnce.Do(func() {
		var loaded bool
		d.dialer, loaded = d.router.Outbound(d.options.Detour)
		if !loaded {
			d.initErr = E.New("outbound detour not found: ", d.options.Detour)
		}
	})
	return d.dialer, d.initErr
}

func (d *lazyDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	dialer, err := d.Dialer()
	if err != nil {
		return nil, err
	}
	return dialer.DialContext(ctx, network, destination)
}

func (d *lazyDialer) ListenPacket(ctx context.Context) (net.PacketConn, error) {
	dialer, err := d.Dialer()
	if err != nil {
		return nil, err
	}
	return dialer.ListenPacket(ctx)
}

func CopyEarlyConn(ctx context.Context, conn net.Conn, serverConn net.Conn) error {
	_payload := buf.StackNew()
	payload := common.Dup(_payload)
	err := conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	if err != nil {
		return err
	}
	_, err = payload.ReadFrom(conn)
	if err != nil && !E.IsTimeout(err) {
		return E.Cause(err, "read payload")
	}
	err = conn.SetReadDeadline(time.Time{})
	if err != nil {
		payload.Release()
		return err
	}
	_, err = serverConn.Write(payload.Bytes())
	if err != nil {
		return E.Cause(err, "client handshake")
	}
	runtime.KeepAlive(_payload)
	return bufio.CopyConn(ctx, conn, serverConn)
}
