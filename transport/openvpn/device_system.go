package openvpn

import (
	"context"
	"net"
	"net/netip"
	"runtime"
	"slices"
	"sync"
	"syscall"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing-tun/gtcpip/header"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

var _ Device = (*systemDevice)(nil)

const (
	systemDeviceReadBufferSize  = 65535 + tun.PacketOffset
	systemDevicePacketRearSpace = 64
)

type systemDevice struct {
	baseDevice
	stateAccess     sync.RWMutex
	options         DeviceOptions
	dialer          N.Dialer
	device          tun.Tun
	inet4Address    netip.Addr
	inet6Address    netip.Addr
	logRouteOptions bool
	closed          bool
}

func newSystemDevice(options DeviceOptions) (*systemDevice, error) {
	if options.Name == "" {
		options.Name = tun.CalculateInterfaceName("ovpn")
	}
	if options.MTU == 0 {
		options.MTU = DefaultMTU
	}
	interfaceDialer, err := dialer.NewDefault(options.Context, option.DialerOptions{
		BindInterface: options.Name,
	})
	if err != nil {
		return nil, err
	}
	inet4Address, inet6Address := firstAddresses(options.Configuration.Address)
	return &systemDevice{
		options:         options,
		dialer:          interfaceDialer,
		inet4Address:    inet4Address,
		inet6Address:    inet6Address,
		logRouteOptions: true,
	}, nil
}

func (d *systemDevice) Start() error {
	d.stateAccess.Lock()
	defer d.stateAccess.Unlock()
	return d.startLocked()
}

func (d *systemDevice) startLocked() error {
	if d.closed {
		return net.ErrClosed
	}
	if d.device != nil {
		return nil
	}
	tunOptions := d.buildTunOptions()
	tunInterface, err := tun.New(tunOptions)
	if err != nil {
		return err
	}
	err = tunInterface.Start()
	if err != nil {
		tunInterface.Close()
		return err
	}
	d.device = tunInterface
	d.options.Logger.Info("started at ", d.options.Name)
	go d.readLoop(tunInterface, int(d.options.MTU))
	return nil
}

func (d *systemDevice) buildTunOptions() tun.Options {
	inet4Address, inet6Address := firstAddresses(d.options.Configuration.Address)
	d.inet4Address = inet4Address
	d.inet6Address = inet6Address
	inet4Addresses, inet6Addresses := splitPrefixes(d.options.Configuration.Address)
	if d.options.Configuration.BlockIPv6 && len(inet6Addresses) == 0 {
		inet6Addresses = append(inet6Addresses, netip.MustParsePrefix("fddd:1194:1194:1194::2/64"))
	}
	routes := routesWithBlockIPv6(d.options.Configuration)
	inet4Routes, inet6Routes := splitRoutes(routes)
	inet4Gateway, _ := systemRouteGateway(routes, true)
	inet6Gateway, _ := systemRouteGateway(routes, false)
	networkManager := service.FromContext[adapter.NetworkManager](d.options.Context)
	tunOptions := tun.Options{
		Name:                 d.options.Name,
		Inet4Address:         inet4Addresses,
		Inet6Address:         inet6Addresses,
		MTU:                  d.options.MTU,
		GSO:                  true,
		InterfaceScope:       true,
		DNSAddress:           d.options.Configuration.DNS,
		Inet4Gateway:         inet4Gateway,
		Inet6Gateway:         inet6Gateway,
		Inet4RouteAddress:    inet4Routes,
		Inet6RouteAddress:    inet6Routes,
		InterfaceMonitor:     nil,
		InterfaceFinder:      nil,
		Logger:               d.options.Logger,
		IPRoute2TableIndex:   tun.DefaultIPRoute2TableIndex,
		IPRoute2RuleIndex:    tun.DefaultIPRoute2RuleIndex,
		EXP_DisableDNSHijack: true,
	}
	if runtime.GOOS == "darwin" {
		tunOptions.AutoRoute = true
	}
	if networkManager != nil {
		tunOptions.InterfaceMonitor = networkManager.InterfaceMonitor()
		tunOptions.InterfaceFinder = networkManager.InterfaceFinder()
	}
	return tunOptions
}

func systemRouteGateway(routes []Route, ipv4 bool) (netip.Addr, bool) {
	var gateway netip.Addr
	var hasGateway bool
	var hasMissingGateway bool
	var gatewayUnrepresentable bool
	var metricUnrepresentable bool
	for _, route := range routes {
		if route.Prefix.Addr().Is4() != ipv4 {
			continue
		}
		if route.Metric != 0 {
			metricUnrepresentable = true
		}
		if !route.Gateway.IsValid() {
			hasMissingGateway = true
			continue
		}
		if route.Gateway.Is4() != ipv4 {
			gatewayUnrepresentable = true
			continue
		}
		if !hasGateway {
			gateway = route.Gateway
			hasGateway = true
		} else if gateway != route.Gateway {
			gatewayUnrepresentable = true
		}
	}
	if hasGateway && hasMissingGateway {
		gatewayUnrepresentable = true
	}
	if gatewayUnrepresentable {
		gateway = netip.Addr{}
	}
	return gateway, gatewayUnrepresentable || metricUnrepresentable
}

func (d *systemDevice) readLoop(tunInterface tun.Tun, mtu int) {
	linuxTUN, isLinuxTUN := tunInterface.(tun.LinuxTUN)
	if isLinuxTUN && linuxTUN.BatchSize() > 1 {
		d.readLoopLinux(linuxTUN, linuxTUN.BatchSize(), mtu)
		return
	}
	darwinTUN, isDarwinTUN := tunInterface.(tun.DarwinTUN)
	if isDarwinTUN {
		d.readLoopDarwin(darwinTUN)
		return
	}
	packetBuffer := buf.NewSize(PacketHeadroom + systemDeviceReadBufferSize + systemDevicePacketRearSpace)
	defer packetBuffer.Release()
	for {
		packetBuffer.Reset()
		packetBuffer.Resize(PacketHeadroom, 0)
		readN, err := tunInterface.Read(packetBuffer.FreeBytes()[:systemDeviceReadBufferSize])
		if err != nil {
			if E.IsClosed(err) {
				return
			}
			d.options.Logger.Error(E.Cause(err, "read packet"))
			return
		}
		if readN <= tun.PacketOffset {
			continue
		}
		packetBuffer.Truncate(readN)
		packetBuffer.Advance(tun.PacketOffset)
		if d.blockIPv6Enabled() && header.IPVersion(packetBuffer.Bytes()) == header.IPv6Version {
			continue
		}
		packetBuffer.IncRef()
		err = d.writeOutbound([]*buf.Buffer{packetBuffer})
		packetBuffer.DecRef()
		if err != nil {
			d.options.Logger.Error(E.Cause(err, "write packet"))
			return
		}
	}
}

func (d *systemDevice) readLoopLinux(tunInterface tun.LinuxTUN, batchSize int, mtu int) {
	packetBuffers := make([]*buf.Buffer, batchSize)
	readBuffers := make([][]byte, batchSize)
	packetSizes := make([]int, batchSize)
	outboundBuffers := make([]*buf.Buffer, 0, batchSize)
	for i := range packetBuffers {
		packetBuffers[i] = buf.NewSize(PacketHeadroom + mtu + systemDevicePacketRearSpace)
	}
	defer buf.ReleaseMulti(packetBuffers)
	for {
		for i, packetBuffer := range packetBuffers {
			packetBuffer.Reset()
			packetBuffer.Resize(PacketHeadroom, 0)
			readBuffers[i] = packetBuffer.FreeBytes()[:mtu]
		}
		packetCount, readErr := tunInterface.BatchRead(readBuffers, 0, packetSizes)
		outboundBuffers = outboundBuffers[:0]
		blockIPv6 := d.blockIPv6Enabled()
		for i := range packetCount {
			packetBuffers[i].Truncate(packetSizes[i])
			if blockIPv6 && header.IPVersion(packetBuffers[i].Bytes()) == header.IPv6Version {
				continue
			}
			packetBuffers[i].IncRef()
			outboundBuffers = append(outboundBuffers, packetBuffers[i])
		}
		if len(outboundBuffers) > 0 {
			writeErr := d.writeOutbound(outboundBuffers)
			for _, packetBuffer := range outboundBuffers {
				packetBuffer.DecRef()
			}
			if writeErr != nil {
				d.options.Logger.Error(E.Cause(writeErr, "write packet batch"))
				return
			}
		}
		if readErr != nil {
			if E.IsClosed(readErr) {
				return
			}
			d.options.Logger.Error(E.Cause(readErr, "batch read packet"))
			return
		}
	}
}

func (d *systemDevice) readLoopDarwin(tunInterface tun.DarwinTUN) {
	for {
		packetBuffers, readErr := tunInterface.BatchRead()
		outboundBuffers := packetBuffers[:0]
		blockIPv6 := d.blockIPv6Enabled()
		for _, packetBuffer := range packetBuffers {
			if packetBuffer.IsEmpty() {
				packetBuffer.Release()
				continue
			}
			if blockIPv6 && header.IPVersion(packetBuffer.Bytes()) == header.IPv6Version {
				packetBuffer.Release()
				continue
			}
			outboundBuffers = append(outboundBuffers, packetBuffer)
		}
		if len(outboundBuffers) > 0 {
			writeErr := d.writeOutbound(outboundBuffers)
			if writeErr != nil {
				d.options.Logger.Error(E.Cause(writeErr, "write packet batch"))
				return
			}
		}
		if readErr != nil {
			if E.IsClosed(readErr) || E.IsMulti(readErr, syscall.EBADF) {
				return
			}
			d.options.Logger.Error(E.Cause(readErr, "batch read packet"))
			return
		}
	}
}

func (d *systemDevice) UpdateConfiguration(configuration Configuration) error {
	d.stateAccess.Lock()
	defer d.stateAccess.Unlock()
	routes := routesWithBlockIPv6(configuration)
	_, hasUnrepresentableInet4RouteOptions := systemRouteGateway(routes, true)
	_, hasUnrepresentableInet6RouteOptions := systemRouteGateway(routes, false)
	if d.logRouteOptions && (hasUnrepresentableInet4RouteOptions || hasUnrepresentableInet6RouteOptions) {
		d.options.Logger.Debug("some OpenVPN route gateway or metric options are not representable by the system device; routes are installed by prefix")
		d.logRouteOptions = false
	}
	previousConfiguration := d.options.Configuration
	previousMTU := d.options.MTU
	updatedMTU := d.options.MTU
	if configuration.MTU != 0 {
		updatedMTU = configuration.MTU
	}
	d.options.MTU = updatedMTU
	d.options.Configuration = configuration
	if d.device == nil {
		inet4Address, inet6Address := firstAddresses(configuration.Address)
		d.inet4Address = inet4Address
		d.inet6Address = inet6Address
		return nil
	}
	if !slices.Equal(previousConfiguration.Address, configuration.Address) ||
		previousMTU != updatedMTU ||
		!slices.Equal(previousConfiguration.DNS, configuration.DNS) ||
		previousConfiguration.BlockIPv6 != configuration.BlockIPv6 {
		d.device.Close()
		d.device = nil
		return d.startLocked()
	}
	return d.device.UpdateRouteOptions(d.buildTunOptions())
}

func (d *systemDevice) blockIPv6Enabled() bool {
	d.stateAccess.RLock()
	defer d.stateAccess.RUnlock()
	return d.options.Configuration.BlockIPv6
}

func (d *systemDevice) WriteInboundBuffers(packetBuffers []*buf.Buffer) error {
	return d.processInboundBuffers(packetBuffers, d.writeBuffers)
}

func (d *systemDevice) writeBuffers(packetBuffers []*buf.Buffer) error {
	d.stateAccess.RLock()
	tunInterface := d.device
	d.stateAccess.RUnlock()
	if tunInterface == nil {
		return E.New("OpenVPN system device is not ready")
	}
	linuxTUN, isLinuxTUN := tunInterface.(tun.LinuxTUN)
	if isLinuxTUN {
		headroom := linuxTUN.FrontHeadroom()
		packets := make([][]byte, len(packetBuffers))
		var temporaryBuffers []*buf.Buffer
		for i, packetBuffer := range packetBuffers {
			if packetBuffer.Start() >= headroom {
				packetBuffer.ExtendHeader(headroom)
				packets[i] = packetBuffer.Bytes()
				packetBuffer.Advance(headroom)
				continue
			}
			temporaryBuffer := buf.NewSize(headroom + packetBuffer.Len())
			temporaryBuffer.Resize(headroom, 0)
			_, _ = temporaryBuffer.Write(packetBuffer.Bytes())
			temporaryBuffer.ExtendHeader(headroom)
			packets[i] = temporaryBuffer.Bytes()
			temporaryBuffers = append(temporaryBuffers, temporaryBuffer)
		}
		_, err := linuxTUN.BatchWrite(packets, headroom)
		buf.ReleaseMulti(temporaryBuffers)
		return err
	}
	darwinTUN, isDarwinTUN := tunInterface.(tun.DarwinTUN)
	if isDarwinTUN {
		return darwinTUN.BatchWrite(packetBuffers)
	}
	for _, packetBuffer := range packetBuffers {
		err := d.writePacket(packetBuffer.Bytes())
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *systemDevice) writePacket(packet []byte) error {
	d.stateAccess.RLock()
	tunInterface := d.device
	d.stateAccess.RUnlock()
	if tunInterface == nil {
		return E.New("OpenVPN system device is not ready")
	}
	if tun.PacketOffset == 0 {
		_, err := tunInterface.Write(packet)
		return err
	}
	writeBuffer := make([]byte, tun.PacketOffset+len(packet))
	tun.PacketFillHeader(writeBuffer[:tun.PacketOffset], header.IPVersion(packet))
	copy(writeBuffer[tun.PacketOffset:], packet)
	_, err := tunInterface.Write(writeBuffer)
	return err
}

func (d *systemDevice) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if !destination.Addr.IsValid() {
		return nil, E.New("invalid non-IP destination")
	}
	return d.dialer.DialContext(ctx, network, destination)
}

func (d *systemDevice) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	if !destination.Addr.IsValid() {
		return nil, E.New("invalid non-IP destination")
	}
	return d.dialer.ListenPacket(ctx, destination)
}

func (d *systemDevice) PortAddresses() (netip.Addr, netip.Addr) {
	d.stateAccess.RLock()
	defer d.stateAccess.RUnlock()
	return d.inet4Address, d.inet6Address
}

func (d *systemDevice) PortMTU() uint32 {
	d.stateAccess.RLock()
	defer d.stateAccess.RUnlock()
	return d.options.MTU
}

func (d *systemDevice) Close() error {
	d.stateAccess.Lock()
	defer d.stateAccess.Unlock()
	d.closed = true
	if d.device == nil {
		return nil
	}
	err := d.device.Close()
	d.device = nil
	return err
}

func (d *systemDevice) configurationAddresses() []netip.Prefix {
	d.stateAccess.RLock()
	defer d.stateAccess.RUnlock()
	return slices.Clone(d.options.Configuration.Address)
}
