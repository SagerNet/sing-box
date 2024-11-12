package wireguard

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/wireguard"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/pause"
	"github.com/sagernet/wireguard-go/conn"
	"github.com/sagernet/wireguard-go/device"
)

func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register[option.WireGuardOutboundOptions](registry, C.TypeWireGuard, NewOutbound)
}

var _ adapter.InterfaceUpdateListener = (*Outbound)(nil)

type Outbound struct {
	outbound.Adapter
	ctx           context.Context
	router        adapter.Router
	logger        logger.ContextLogger
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

func NewOutbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.WireGuardOutboundOptions) (adapter.Outbound, error) {
	outbound := &Outbound{
		Adapter:      outbound.NewAdapterWithDialerOptions(C.TypeWireGuard, options.Network.Build(), tag, options.DialerOptions),
		ctx:          ctx,
		router:       router,
		logger:       logger,
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
	listener, err := dialer.New(ctx, options.DialerOptions)
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
		wireTunDevice, err = wireguard.NewSystemDevice(service.FromContext[adapter.NetworkManager](ctx), options.InterfaceName, options.LocalAddress, mtu, options.GSO)
	}
	if err != nil {
		return nil, E.Cause(err, "create WireGuard device")
	}
	outbound.tunDevice = wireTunDevice
	return outbound, nil
}

func (w *Outbound) Start() error {
	if common.Any(w.peers, func(peer wireguard.PeerConfig) bool {
		return !peer.Endpoint.IsValid()
	}) {
		// wait for all outbounds to be started and continue in PortStart
		return nil
	}
	return w.start()
}

func (w *Outbound) PostStart() error {
	if common.All(w.peers, func(peer wireguard.PeerConfig) bool {
		return peer.Endpoint.IsValid()
	}) {
		return nil
	}
	return w.start()
}

func (w *Outbound) start() error {
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
		bind = wireguard.NewClientBind(w.ctx, w.logger, w.listener, isConnect, connectAddr, reserved)
	}
	if w.useStdNetBind || len(w.peers) > 1 {
		for _, peer := range w.peers {
			if peer.Reserved != [3]uint8{} {
				bind.SetReservedForEndpoint(peer.Endpoint, peer.Reserved)
			}
		}
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

func (w *Outbound) Close() error {
	if w.device != nil {
		w.device.Close()
	}
	if w.pauseCallback != nil {
		w.pauseManager.UnregisterCallback(w.pauseCallback)
	}
	return nil
}

func (w *Outbound) InterfaceUpdated() {
	w.device.BindUpdate()
	return
}

func (w *Outbound) onPauseUpdated(event int) {
	switch event {
	case pause.EventDevicePaused:
		w.device.Down()
	case pause.EventDeviceWake:
		w.device.Up()
	}
}

func (w *Outbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
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

func (w *Outbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
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
