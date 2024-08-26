//go:build with_wireguard

package outbound

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/wireguard"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/pause"
	"github.com/sagernet/wireguard-go/conn"
	"github.com/sagernet/wireguard-go/device"
)

var (
	_ adapter.Outbound                = (*WireGuard)(nil)
	_ adapter.InterfaceUpdateListener = (*WireGuard)(nil)
)

type WireGuard struct {
	myOutboundAdapter
	ctx           context.Context
	workers       int
	peers         []wireguard.PeerConfig
	useStdNetBind bool
	listener      N.Dialer
	ipcConf       string

	pauseManager  pause.Manager
	pauseCallback *list.Element[pause.Callback]
	bind          conn.Bind
	device        *device.Device
	tunDevice     wireguard.Device
}

func NewWireGuard(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.WireGuardOutboundOptions) (*WireGuard, error) {
	outbound := &WireGuard{
		myOutboundAdapter: myOutboundAdapter{
			protocol:     C.TypeWireGuard,
			network:      options.Network.Build(),
			router:       router,
			logger:       logger,
			tag:          tag,
			dependencies: withDialerDependency(options.DialerOptions),
		},
		ctx:          ctx,
		workers:      options.Workers,
		pauseManager: service.FromContext[pause.Manager](ctx),
	}
	peers, err := wireguard.ParsePeers(options)
	if err != nil {
		return nil, err
	}
	outbound.peers = peers
	if len(options.LocalAddress) == 0 {
		return nil, E.New("missing local address")
	}
	if options.GSO {
		if options.GSO && options.Detour != "" {
			return nil, E.New("gso is conflict with detour")
		}
		options.IsWireGuardListener = true
		outbound.useStdNetBind = true
	}
	listener, err := dialer.New(router, options.DialerOptions)
	if err != nil {
		return nil, err
	}
	outbound.listener = listener
	var privateKey string
	{
		bytes, err := base64.StdEncoding.DecodeString(options.PrivateKey)
		if err != nil {
			return nil, E.Cause(err, "decode private key")
		}
		privateKey = hex.EncodeToString(bytes)
	}
	outbound.ipcConf = "private_key=" + privateKey
	mtu := options.MTU
	if mtu == 0 {
		mtu = 1408
	}
	var wireTunDevice wireguard.Device
	if !options.SystemInterface && tun.WithGVisor {
		wireTunDevice, err = wireguard.NewStackDevice(options.LocalAddress, mtu)
	} else {
		wireTunDevice, err = wireguard.NewSystemDevice(router, options.InterfaceName, options.LocalAddress, mtu, options.GSO)
	}
	if err != nil {
		return nil, E.Cause(err, "create WireGuard device")
	}
	outbound.tunDevice = wireTunDevice
	return outbound, nil
}

func (w *WireGuard) Start() error {
	if common.Any(w.peers, func(peer wireguard.PeerConfig) bool {
		return !peer.Endpoint.IsValid()
	}) {
		// wait for all outbounds to be started and continue in PortStart
		return nil
	}
	return w.start()
}

func (w *WireGuard) PostStart() error {
	if common.All(w.peers, func(peer wireguard.PeerConfig) bool {
		return peer.Endpoint.IsValid()
	}) {
		return nil
	}
	return w.start()
}

func (w *WireGuard) start() error {
	err := wireguard.ResolvePeers(w.ctx, w.router, w.peers)
	if err != nil {
		return err
	}
	var bind conn.Bind
	if w.useStdNetBind {
		bind = conn.NewStdNetBind(w.listener.(dialer.WireGuardListener))
	} else {
		var (
			isConnect   bool
			connectAddr netip.AddrPort
			reserved    [3]uint8
		)
		peerLen := len(w.peers)
		if peerLen == 1 {
			isConnect = true
			connectAddr = w.peers[0].Endpoint
			reserved = w.peers[0].Reserved
		}
		bind = wireguard.NewClientBind(w.ctx, w, w.listener, isConnect, connectAddr, reserved)
	}
	err = w.tunDevice.Start()
	if err != nil {
		return err
	}
	wgDevice := device.NewDevice(w.tunDevice, bind, &device.Logger{
		Verbosef: func(format string, args ...interface{}) {
			w.logger.Debug(fmt.Sprintf(strings.ToLower(format), args...))
		},
		Errorf: func(format string, args ...interface{}) {
			w.logger.Error(fmt.Sprintf(strings.ToLower(format), args...))
		},
	}, w.workers)
	ipcConf := w.ipcConf
	for _, peer := range w.peers {
		ipcConf += peer.GenerateIpcLines()
	}
	err = wgDevice.IpcSet(ipcConf)
	if err != nil {
		return E.Cause(err, "setup wireguard: \n", ipcConf)
	}
	w.device = wgDevice
	w.pauseCallback = w.pauseManager.RegisterCallback(w.onPauseUpdated)
	return nil
}

func (w *WireGuard) Close() error {
	if w.device != nil {
		w.device.Close()
	}
	if w.pauseCallback != nil {
		w.pauseManager.UnregisterCallback(w.pauseCallback)
	}
	return nil
}

func (w *WireGuard) InterfaceUpdated() {
	w.device.BindUpdate()
	return
}

func (w *WireGuard) onPauseUpdated(event int) {
	switch event {
	case pause.EventDevicePaused:
		w.device.Down()
	case pause.EventDeviceWake:
		w.device.Up()
	}
}

func (w *WireGuard) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	switch network {
	case N.NetworkTCP:
		w.logger.InfoContext(ctx, "outbound connection to ", destination)
	case N.NetworkUDP:
		w.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	}
	if destination.IsFqdn() {
		destinationAddresses, err := w.router.LookupDefault(ctx, destination.Fqdn)
		if err != nil {
			return nil, err
		}
		return N.DialSerial(ctx, w.tunDevice, network, destination, destinationAddresses)
	}
	return w.tunDevice.DialContext(ctx, network, destination)
}

func (w *WireGuard) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	w.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	if destination.IsFqdn() {
		destinationAddresses, err := w.router.LookupDefault(ctx, destination.Fqdn)
		if err != nil {
			return nil, err
		}
		packetConn, _, err := N.ListenSerial(ctx, w.tunDevice, destination, destinationAddresses)
		if err != nil {
			return nil, err
		}
		return packetConn, err
	}
	return w.tunDevice.ListenPacket(ctx, destination)
}

func (w *WireGuard) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return NewDirectConnection(ctx, w.router, w, conn, metadata, dns.DomainStrategyAsIS)
}

func (w *WireGuard) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return NewDirectPacketConnection(ctx, w.router, w, conn, metadata, dns.DomainStrategyAsIS)
}
