package wireguard

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	wgTun "github.com/sagernet/wireguard-go/tun"
)

var _ Device = (*SystemDevice)(nil)

type SystemDevice struct {
	dialer        N.Dialer
	device        tun.Tun
	frontHeadroom int
	name          string
	mtu           int
	events        chan wgTun.Event
	addr4         netip.Addr
	addr6         netip.Addr
}

func NewSystemDevice(router adapter.Router, interfaceName string, localPrefixes []netip.Prefix, mtu uint32, gso bool, gsoMaxsize uint32) (*SystemDevice, error) {
	var inet4Addresses []netip.Prefix
	var inet6Addresses []netip.Prefix
	for _, prefixes := range localPrefixes {
		if prefixes.Addr().Is4() {
			inet4Addresses = append(inet4Addresses, prefixes)
		} else {
			inet6Addresses = append(inet6Addresses, prefixes)
		}
	}
	if interfaceName == "" {
		interfaceName = tun.CalculateInterfaceName("wg")
	}
	if gsoMaxsize == 0 {
		gsoMaxsize = 65536
	}
	tunInterface, err := tun.New(tun.Options{
		Name:         interfaceName,
		Inet4Address: inet4Addresses,
		Inet6Address: inet6Addresses,
		MTU:          mtu,
		GSO:          gso,
		GSOMaxSize:   gsoMaxsize,
	})
	if err != nil {
		return nil, err
	}
	var inet4Address netip.Addr
	var inet6Address netip.Addr
	if len(inet4Addresses) > 0 {
		inet4Address = inet4Addresses[0].Addr()
	}
	if len(inet6Addresses) > 0 {
		inet6Address = inet6Addresses[0].Addr()
	}
	return &SystemDevice{
		dialer: common.Must1(dialer.NewDefault(router, option.DialerOptions{
			BindInterface: interfaceName,
		})),
		device:        tunInterface,
		frontHeadroom: tunInterface.FrontHeadroom(),
		name:          interfaceName,
		mtu:           int(mtu),
		events:        make(chan wgTun.Event),
		addr4:         inet4Address,
		addr6:         inet6Address,
	}, nil
}

func (w *SystemDevice) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return w.dialer.DialContext(ctx, network, destination)
}

func (w *SystemDevice) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return w.dialer.ListenPacket(ctx, destination)
}

func (w *SystemDevice) Inet4Address() netip.Addr {
	return w.addr4
}

func (w *SystemDevice) Inet6Address() netip.Addr {
	return w.addr6
}

func (w *SystemDevice) Start() error {
	w.events <- wgTun.EventUp
	return nil
}

func (w *SystemDevice) File() *os.File {
	return nil
}

func (w *SystemDevice) Read(bufs [][]byte, sizes []int, offset int) (count int, err error) {
	sizes[0], err = w.device.Read(bufs[0][offset-w.frontHeadroom:])
	if err == nil {
		count = 1
	} else if errors.Is(err, tun.ErrTooManySegments) {
		err = wgTun.ErrTooManySegments
	}
	return
}

func (w *SystemDevice) Write(bufs [][]byte, offset int) (count int, err error) {
	for _, b := range bufs {
		_, err = w.device.Write(b[offset-w.frontHeadroom:])
		if err != nil {
			return
		}
		count++
	}
	return
}

func (w *SystemDevice) Flush() error {
	return nil
}

func (w *SystemDevice) MTU() (int, error) {
	return w.mtu, nil
}

func (w *SystemDevice) Name() (string, error) {
	return w.name, nil
}

func (w *SystemDevice) Events() <-chan wgTun.Event {
	return w.events
}

func (w *SystemDevice) Close() error {
	return w.device.Close()
}

func (w *SystemDevice) BatchSize() int {
	return 1
}
