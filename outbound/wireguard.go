//go:build with_wireguard

package outbound

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/wireguard"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/debug"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/wireguard-go/device"
)

var (
	_ adapter.Outbound                = (*WireGuard)(nil)
	_ adapter.InterfaceUpdateListener = (*WireGuard)(nil)
)

type WireGuard struct {
	myOutboundAdapter
	bind      *wireguard.ClientBind
	device    *device.Device
	tunDevice wireguard.Device
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
	}
	var reserved [3]uint8
	if len(options.Reserved) > 0 {
		if len(options.Reserved) != 3 {
			return nil, E.New("invalid reserved value, required 3 bytes, got ", len(options.Reserved))
		}
		copy(reserved[:], options.Reserved)
	}
	var isConnect bool
	var connectAddr M.Socksaddr
	if len(options.Peers) < 2 {
		isConnect = true
		if len(options.Peers) == 1 {
			connectAddr = options.Peers[0].ServerOptions.Build()
		} else {
			connectAddr = options.ServerOptions.Build()
		}
	}
	outboundDialer, err := dialer.New(router, options.DialerOptions)
	if err != nil {
		return nil, err
	}
	outbound.bind = wireguard.NewClientBind(ctx, outbound, outboundDialer, isConnect, connectAddr, reserved)
	if len(options.LocalAddress) == 0 {
		return nil, E.New("missing local address")
	}
	var privateKey string
	{
		bytes, err := base64.StdEncoding.DecodeString(options.PrivateKey)
		if err != nil {
			return nil, E.Cause(err, "decode private key")
		}
		privateKey = hex.EncodeToString(bytes)
	}
	ipcConf := "private_key=" + privateKey
	if len(options.Peers) > 0 {
		for i, peer := range options.Peers {
			var peerPublicKey, preSharedKey string
			{
				bytes, err := base64.StdEncoding.DecodeString(peer.PublicKey)
				if err != nil {
					return nil, E.Cause(err, "decode public key for peer ", i)
				}
				peerPublicKey = hex.EncodeToString(bytes)
			}
			if peer.PreSharedKey != "" {
				bytes, err := base64.StdEncoding.DecodeString(peer.PreSharedKey)
				if err != nil {
					return nil, E.Cause(err, "decode pre shared key for peer ", i)
				}
				preSharedKey = hex.EncodeToString(bytes)
			}
			destination := peer.ServerOptions.Build()
			ipcConf += "\npublic_key=" + peerPublicKey
			ipcConf += "\nendpoint=" + destination.String()
			if preSharedKey != "" {
				ipcConf += "\npreshared_key=" + preSharedKey
			}
			if len(peer.AllowedIPs) == 0 {
				return nil, E.New("missing allowed_ips for peer ", i)
			}
			for _, allowedIP := range peer.AllowedIPs {
				ipcConf += "\nallowed_ip=" + allowedIP
			}
			if len(peer.Reserved) > 0 {
				if len(peer.Reserved) != 3 {
					return nil, E.New("invalid reserved value for peer ", i, ", required 3 bytes, got ", len(peer.Reserved))
				}
				copy(reserved[:], options.Reserved)
				outbound.bind.SetReservedForEndpoint(destination, reserved)
			}
		}
	} else {
		var peerPublicKey, preSharedKey string
		{
			bytes, err := base64.StdEncoding.DecodeString(options.PeerPublicKey)
			if err != nil {
				return nil, E.Cause(err, "decode peer public key")
			}
			peerPublicKey = hex.EncodeToString(bytes)
		}
		if options.PreSharedKey != "" {
			bytes, err := base64.StdEncoding.DecodeString(options.PreSharedKey)
			if err != nil {
				return nil, E.Cause(err, "decode pre shared key")
			}
			preSharedKey = hex.EncodeToString(bytes)
		}
		ipcConf += "\npublic_key=" + peerPublicKey
		ipcConf += "\nendpoint=" + options.ServerOptions.Build().String()
		if preSharedKey != "" {
			ipcConf += "\npreshared_key=" + preSharedKey
		}
		var has4, has6 bool
		for _, address := range options.LocalAddress {
			if address.Addr().Is4() {
				has4 = true
			} else {
				has6 = true
			}
		}
		if has4 {
			ipcConf += "\nallowed_ip=0.0.0.0/0"
		}
		if has6 {
			ipcConf += "\nallowed_ip=::/0"
		}
	}
	mtu := options.MTU
	if mtu == 0 {
		mtu = 1408
	}
	var wireTunDevice wireguard.Device
	if !options.SystemInterface && tun.WithGVisor {
		wireTunDevice, err = wireguard.NewStackDevice(options.LocalAddress, mtu)
	} else {
		wireTunDevice, err = wireguard.NewSystemDevice(router, options.InterfaceName, options.LocalAddress, mtu, options.GSO, options.GSOMaxSize)
	}
	if err != nil {
		return nil, E.Cause(err, "create WireGuard device")
	}
	wgDevice := device.NewDevice(ctx, wireTunDevice, outbound.bind, &device.Logger{
		Verbosef: func(format string, args ...interface{}) {
			logger.Debug(fmt.Sprintf(strings.ToLower(format), args...))
		},
		Errorf: func(format string, args ...interface{}) {
			logger.Error(fmt.Sprintf(strings.ToLower(format), args...))
		},
	}, options.Workers)
	if debug.Enabled {
		logger.Trace("created wireguard ipc conf: \n", ipcConf)
	}
	err = wgDevice.IpcSet(ipcConf)
	if err != nil {
		return nil, E.Cause(err, "setup wireguard")
	}
	outbound.device = wgDevice
	outbound.tunDevice = wireTunDevice
	return outbound, nil
}

func (w *WireGuard) InterfaceUpdated() {
	w.bind.Reset()
	return
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

func (w *WireGuard) Start() error {
	return w.tunDevice.Start()
}

func (w *WireGuard) Close() error {
	if w.device != nil {
		w.device.Close()
	}
	w.tunDevice.Close()
	return nil
}
