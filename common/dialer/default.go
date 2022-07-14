package dialer

import (
	"context"
	"net"
	"runtime"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/control"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/database64128/tfo-go"
)

type DefaultDialer struct {
	tfo.Dialer
	net.ListenConfig
}

func NewDefault(router adapter.Router, options option.DialerOptions) *DefaultDialer {
	var dialer net.Dialer
	var listener net.ListenConfig
	if options.BindInterface != "" {
		dialer.Control = control.Append(dialer.Control, control.BindToInterface(router.InterfaceBindManager(), options.BindInterface))
		listener.Control = control.Append(listener.Control, control.BindToInterface(router.InterfaceBindManager(), options.BindInterface))
	} else if router.AutoDetectInterface() {
		if runtime.GOOS == "windows" {
			dialer.Control = control.Append(dialer.Control, control.BindToInterfaceIndexFunc(func() int {
				return router.DefaultInterfaceIndex()
			}))
			listener.Control = control.Append(listener.Control, control.BindToInterfaceIndexFunc(func() int {
				return router.DefaultInterfaceIndex()
			}))
		} else {
			dialer.Control = control.Append(dialer.Control, control.BindToInterfaceFunc(router.InterfaceBindManager(), func() string {
				return router.DefaultInterfaceName()
			}))
			listener.Control = control.Append(listener.Control, control.BindToInterfaceFunc(router.InterfaceBindManager(), func() string {
				return router.DefaultInterfaceName()
			}))
		}
	}
	if options.RoutingMark != 0 {
		dialer.Control = control.Append(dialer.Control, control.RoutingMark(options.RoutingMark))
		listener.Control = control.Append(listener.Control, control.RoutingMark(options.RoutingMark))
	}
	if options.ReuseAddr {
		listener.Control = control.Append(listener.Control, control.ReuseAddr())
	}
	if options.ProtectPath != "" {
		dialer.Control = control.Append(dialer.Control, control.ProtectPath(options.ProtectPath))
		listener.Control = control.Append(listener.Control, control.ProtectPath(options.ProtectPath))
	}
	if options.ConnectTimeout != 0 {
		dialer.Timeout = time.Duration(options.ConnectTimeout)
	}
	return &DefaultDialer{tfo.Dialer{Dialer: dialer, DisableTFO: !options.TCPFastOpen}, listener}
}

func (d *DefaultDialer) DialContext(ctx context.Context, network string, address M.Socksaddr) (net.Conn, error) {
	return d.Dialer.DialContext(ctx, network, address.String())
}

func (d *DefaultDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return d.ListenConfig.ListenPacket(ctx, C.NetworkUDP, "")
}

func (d *DefaultDialer) Upstream() any {
	return &d.Dialer
}
