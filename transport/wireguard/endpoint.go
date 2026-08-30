package wireguard

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/pause"
	"github.com/sagernet/wireguard-go/conn"
	"github.com/sagernet/wireguard-go/device"

	"go4.org/netipx"
)

type Endpoint struct {
	options          EndpointOptions
	peers            []peerConfig
	ipcConf          string
	allowedAddress   []netip.Prefix
	tunDevice        Device
	natDevice        NatDevice
	device           *device.Device
	allowedIPs       *device.AllowedIPs
	pause            pause.Manager
	pauseCallback    *list.Element[pause.Callback]
	bind             *runtimeBind
	ipcSet           func(string) error
	dnsRefresh       chan struct{}
	dnsRefreshDone   chan struct{}
	dnsRefreshCancel context.CancelFunc
}

const (
	// wireGuardHandshakeRetryLog is emitted by wireguard-go's expiredRetransmitHandshake.
	// wireguard-go does not expose a typed callback for this event.
	wireGuardHandshakeRetryLog = "%s - Handshake did not complete after %d seconds, retrying (try %d)"
	dnsRefreshInterval         = 10 * time.Second
)

func NewEndpoint(options EndpointOptions) (*Endpoint, error) {
	if options.PrivateKey == "" {
		return nil, E.New("missing private key")
	}
	privateKeyBytes, err := base64.StdEncoding.DecodeString(options.PrivateKey)
	if err != nil {
		return nil, E.Cause(err, "decode private key")
	}
	privateKey := hex.EncodeToString(privateKeyBytes)
	ipcConf := "private_key=" + privateKey
	if options.ListenPort != 0 {
		ipcConf += "\nlisten_port=" + F.ToString(options.ListenPort)
	}
	var peers []peerConfig
	for peerIndex, rawPeer := range options.Peers {
		peer := peerConfig{
			allowedIPs: rawPeer.AllowedIPs,
			keepalive:  rawPeer.PersistentKeepaliveInterval,
		}
		if rawPeer.Endpoint.Addr.IsValid() {
			peer.endpoint = rawPeer.Endpoint.AddrPort()
		} else if rawPeer.Endpoint.IsDomain() {
			peer.destination = rawPeer.Endpoint
		}
		publicKeyBytes, err := base64.StdEncoding.DecodeString(rawPeer.PublicKey)
		if err != nil {
			return nil, E.Cause(err, "decode public key for peer ", peerIndex)
		}
		peer.publicKeyHex = hex.EncodeToString(publicKeyBytes)
		if rawPeer.PreSharedKey != "" {
			preSharedKeyBytes, err := base64.StdEncoding.DecodeString(rawPeer.PreSharedKey)
			if err != nil {
				return nil, E.Cause(err, "decode pre shared key for peer ", peerIndex)
			}
			peer.preSharedKeyHex = hex.EncodeToString(preSharedKeyBytes)
		}
		if len(rawPeer.AllowedIPs) == 0 {
			return nil, E.New("missing allowed ips for peer ", peerIndex)
		}
		if len(rawPeer.Reserved) > 0 {
			if len(rawPeer.Reserved) != 3 {
				return nil, E.New("invalid reserved value for peer ", peerIndex, ", required 3 bytes, got ", len(peer.reserved))
			}
			copy(peer.reserved[:], rawPeer.Reserved[:])
		}
		peers = append(peers, peer)
	}
	var allowedPrefixBuilder netipx.IPSetBuilder
	for _, peer := range options.Peers {
		for _, prefix := range peer.AllowedIPs {
			allowedPrefixBuilder.AddPrefix(prefix)
		}
	}
	allowedIPSet, err := allowedPrefixBuilder.IPSet()
	if err != nil {
		return nil, err
	}
	allowedAddresses := allowedIPSet.Prefixes()
	if options.MTU == 0 {
		options.MTU = 1408
	}
	deviceOptions := DeviceOptions{
		Context:        options.Context,
		Logger:         options.Logger,
		System:         options.System,
		Handler:        options.Handler,
		UDPTimeout:     options.UDPTimeout,
		ICMPTimeout:    options.ICMPTimeout,
		CreateDialer:   options.CreateDialer,
		Name:           options.Name,
		MTU:            options.MTU,
		Address:        options.Address,
		AllowedAddress: allowedAddresses,
	}
	tunDevice, err := NewDevice(deviceOptions)
	if err != nil {
		return nil, E.Cause(err, "create WireGuard device")
	}
	natDevice, isNatDevice := tunDevice.(NatDevice)
	if !isNatDevice {
		natDevice = NewNATDevice(options.Context, options.Logger, tunDevice)
	}
	return &Endpoint{
		options:        options,
		peers:          peers,
		ipcConf:        ipcConf,
		allowedAddress: allowedAddresses,
		tunDevice:      tunDevice,
		natDevice:      natDevice,
	}, nil
}

func (e *Endpoint) Start(resolve bool) error {
	if common.Any(e.peers, func(peer peerConfig) bool {
		return !peer.endpoint.IsValid() && peer.destination.IsDomain()
	}) {
		if !resolve {
			return nil
		}
		for peerIndex, peer := range e.peers {
			if peer.endpoint.IsValid() || !peer.destination.IsDomain() {
				continue
			}
			destinationAddresses, err := e.options.ResolvePeer(e.options.Context, peer.destination.Fqdn, false)
			if err != nil {
				return E.Cause(err, "resolve endpoint domain for peer[", peerIndex, "]: ", peer.destination)
			}
			destinationAddress, loaded := firstValidAddress(destinationAddresses)
			if !loaded {
				return E.New("no addresses found for peer[", peerIndex, "]: ", peer.destination)
			}
			e.peers[peerIndex].endpoint = netip.AddrPortFrom(destinationAddress, peer.destination.Port)
		}
	} else if resolve {
		return nil
	}
	var rawBind conn.Bind
	wgListener, isWgListener := common.Cast[dialer.WireGuardListener](e.options.Dialer)
	if isWgListener {
		rawBind = conn.NewStdNetBind(wgListener.WireGuardControl())
	} else {
		var (
			isConnect   bool
			connectAddr netip.AddrPort
			reserved    [3]uint8
		)
		if len(e.peers) == 1 && e.peers[0].endpoint.IsValid() && !e.peers[0].destination.IsDomain() {
			isConnect = true
			connectAddr = e.peers[0].endpoint
			reserved = e.peers[0].reserved
		}
		rawBind = NewClientBind(e.options.Context, e.options.Logger, e.options.Dialer, isConnect, connectAddr, reserved)
	}
	bind := &runtimeBind{Bind: rawBind}
	e.bind = bind
	if isWgListener || len(e.peers) > 1 {
		for _, peer := range e.peers {
			if peer.reserved != [3]uint8{} {
				bind.SetReservedForEndpoint(peer.endpoint, peer.reserved)
			}
		}
	}
	err := e.tunDevice.Start()
	if err != nil {
		return err
	}
	logger := &device.Logger{
		Verbosef: func(format string, args ...any) {
			if isWireGuardHandshakeRetry(format) {
				e.triggerDNSRefresh()
			}
			e.options.Logger.Debug(fmt.Sprintf(strings.ToLower(format), args...))
		},
		Errorf: func(format string, args ...any) {
			e.options.Logger.Error(fmt.Sprintf(strings.ToLower(format), args...))
		},
	}
	var deviceInput Device
	if e.natDevice != nil {
		deviceInput = e.natDevice
	} else {
		deviceInput = e.tunDevice
	}
	wgDevice := device.NewDevice(e.options.Context, deviceInput, bind, logger, e.options.Workers)
	e.tunDevice.SetDevice(wgDevice)
	var ipcConf strings.Builder
	ipcConf.WriteString(e.ipcConf)
	for _, peer := range e.peers {
		ipcConf.WriteString(peer.GenerateIpcLines())
	}
	err = wgDevice.IpcSet(ipcConf.String())
	if err != nil {
		wgDevice.Close()
		return E.Cause(err, "setup wireguard: \n", ipcConf.String())
	}
	e.device = wgDevice
	e.ipcSet = wgDevice.IpcSet
	e.startDNSRefresh()
	e.pause = service.FromContext[pause.Manager](e.options.Context)
	if e.pause != nil {
		e.pauseCallback = e.pause.RegisterCallback(e.onPauseUpdated)
	}
	e.allowedIPs = (*device.AllowedIPs)(unsafe.Pointer(reflect.Indirect(reflect.ValueOf(wgDevice)).FieldByName("allowedips").UnsafeAddr()))
	return nil
}

func (e *Endpoint) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if !destination.Addr.IsValid() {
		return nil, E.Cause(os.ErrInvalid, "invalid non-IP destination")
	}
	return e.tunDevice.DialContext(ctx, network, destination)
}

func (e *Endpoint) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	if !destination.Addr.IsValid() {
		return nil, E.Cause(os.ErrInvalid, "invalid non-IP destination")
	}
	return e.tunDevice.ListenPacket(ctx, destination)
}

func (e *Endpoint) Close() error {
	e.stopDNSRefresh()
	if e.pauseCallback != nil {
		e.pause.UnregisterCallback(e.pauseCallback)
		e.pauseCallback = nil
	}
	if e.device != nil {
		e.device.Down()
		e.device.Close()
		e.device = nil
	}
	e.ipcSet = nil
	return nil
}

func (e *Endpoint) Lookup(address netip.Addr) *device.Peer {
	if e.allowedIPs == nil {
		return nil
	}
	return e.allowedIPs.Lookup(address.AsSlice())
}

func (e *Endpoint) NewDirectRouteConnection(metadata adapter.InboundContext, routeContext tun.DirectRouteContext, timeout time.Duration) (tun.DirectRouteDestination, error) {
	if e.natDevice == nil {
		return nil, os.ErrInvalid
	}
	return e.natDevice.CreateDestination(metadata, routeContext, timeout)
}

func (e *Endpoint) onPauseUpdated(event int) {
	switch event {
	case pause.EventDevicePaused, pause.EventNetworkPause:
		e.device.Down()
	case pause.EventDeviceWake, pause.EventNetworkWake:
		e.device.Up()
	}
}

func (e *Endpoint) startDNSRefresh() {
	if !common.Any(e.peers, func(peer peerConfig) bool { return peer.destination.IsDomain() }) {
		return
	}
	refreshCtx, cancel := context.WithCancel(e.options.Context)
	e.dnsRefresh = make(chan struct{}, 1)
	e.dnsRefreshDone = make(chan struct{})
	e.dnsRefreshCancel = cancel
	go func() {
		defer close(e.dnsRefreshDone)
		var lastRefresh time.Time
		for {
			select {
			case <-refreshCtx.Done():
				return
			case <-e.dnsRefresh:
				if time.Since(lastRefresh) < dnsRefreshInterval {
					continue
				}
				e.refreshPeerEndpoints(refreshCtx)
				lastRefresh = time.Now()
			}
		}
	}()
}

func (e *Endpoint) stopDNSRefresh() {
	if e.dnsRefreshCancel == nil {
		return
	}
	e.dnsRefreshCancel()
	<-e.dnsRefreshDone
	e.dnsRefreshCancel = nil
}

func (e *Endpoint) triggerDNSRefresh() {
	if e.dnsRefresh == nil {
		return
	}
	select {
	case e.dnsRefresh <- struct{}{}:
	default:
	}
}

func (e *Endpoint) refreshPeerEndpoints(ctx context.Context) {
	if e.ipcSet == nil {
		return
	}
	for peerIndex := range e.peers {
		peer := &e.peers[peerIndex]
		if !peer.destination.IsDomain() {
			continue
		}
		addresses, err := e.options.ResolvePeer(ctx, peer.destination.Fqdn, true)
		if err != nil {
			e.options.Logger.Warn(E.Cause(err, "resolve WireGuard peer endpoint: ", peer.destination.Fqdn))
			continue
		}
		newAddress, loaded := firstDifferentAddress(addresses, peer.endpoint.Addr())
		if !loaded {
			continue
		}
		oldEndpoint := peer.endpoint
		newEndpoint := netip.AddrPortFrom(newAddress, peer.destination.Port)
		if peer.reserved != [3]uint8{} && e.bind != nil {
			e.bind.SetReservedForEndpoint(newEndpoint, peer.reserved)
		}
		ipcConf := "public_key=" + peer.publicKeyHex + "\nupdate_only=true\nendpoint=" + newEndpoint.String()
		if err = e.ipcSet(ipcConf); err != nil {
			e.options.Logger.Warn(E.Cause(err, "update WireGuard peer endpoint: ", peer.destination.Fqdn))
			continue
		}
		peer.endpoint = newEndpoint
		e.options.Logger.Info("updated WireGuard peer endpoint for ", peer.destination.Fqdn, ": ", oldEndpoint, " -> ", newEndpoint)
	}
}

func firstValidAddress(addresses []netip.Addr) (netip.Addr, bool) {
	for _, address := range addresses {
		if address.IsValid() {
			return address, true
		}
	}
	return netip.Addr{}, false
}

func firstDifferentAddress(addresses []netip.Addr, current netip.Addr) (netip.Addr, bool) {
	for _, address := range addresses {
		if address.IsValid() && address != current {
			return address, true
		}
	}
	return netip.Addr{}, false
}

func isWireGuardHandshakeRetry(format string) bool {
	return format == wireGuardHandshakeRetryLog
}

type runtimeBind struct {
	conn.Bind
	access sync.RWMutex
}

func (b *runtimeBind) Send(buffers [][]byte, endpoint conn.Endpoint, offset int) error {
	b.access.RLock()
	defer b.access.RUnlock()
	return b.Bind.Send(buffers, endpoint, offset)
}

func (b *runtimeBind) SetReservedForEndpoint(destination netip.AddrPort, reserved [3]byte) {
	b.access.Lock()
	defer b.access.Unlock()
	b.Bind.SetReservedForEndpoint(destination, reserved)
}

type peerConfig struct {
	destination     M.Socksaddr
	endpoint        netip.AddrPort
	publicKeyHex    string
	preSharedKeyHex string
	allowedIPs      []netip.Prefix
	keepalive       uint16
	reserved        [3]uint8
}

func (c peerConfig) GenerateIpcLines() string {
	var ipcLines strings.Builder
	ipcLines.WriteString("\npublic_key=" + c.publicKeyHex)
	if c.endpoint.IsValid() {
		ipcLines.WriteString("\nendpoint=" + c.endpoint.String())
	}
	if c.preSharedKeyHex != "" {
		ipcLines.WriteString("\npreshared_key=" + c.preSharedKeyHex)
	}
	for _, allowedIP := range c.allowedIPs {
		ipcLines.WriteString("\nallowed_ip=" + allowedIP.String())
	}
	if c.keepalive > 0 {
		ipcLines.WriteString("\npersistent_keepalive_interval=" + F.ToString(c.keepalive))
	}
	return ipcLines.String()
}
