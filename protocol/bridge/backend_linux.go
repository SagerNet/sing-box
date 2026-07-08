package bridge

import (
	"context"
	"net/netip"
	"sync"

	"github.com/sagernet/netlink"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/service"

	"golang.org/x/sys/unix"
)

const (
	defaultBridgeRuleIndex      = 100
	defaultBridgeTableIndexBase = 2200
)

type backendLinux struct {
	backendBase

	nftTableName string
	routeTable   int
	ruleIndex    int

	platform adapter.PlatformInterface

	batchTUN tun.LinuxTUN

	writeAccess   sync.Mutex
	writeHeadroom int
	writeBuffers  [][]byte

	clampMTU int
}

func newBackend(ctx context.Context, logger logger.ContextLogger, networkManager adapter.NetworkManager, tag string, options option.BridgeOutboundOptions) (Backend, error) {
	instance := &backendLinux{}
	err := instance.init(ctx, logger, networkManager, tag, options)
	if err != nil {
		return nil, err
	}
	platformInterface := service.FromContext[adapter.PlatformInterface](ctx)
	if platformInterface != nil && platformInterface.UsePlatformBridge() {
		instance.platform = platformInterface
	}
	instance.ruleIndex = options.IPRoute2RuleIndex
	if instance.ruleIndex == 0 {
		instance.ruleIndex = defaultBridgeRuleIndex
	}
	if instance.boundInterface != "" || instance.platform != nil {
		instance.routeTable = options.IPRoute2TableIndex
		if instance.routeTable == 0 {
			instance.routeTable = defaultBridgeTableIndexBase + int(instance.index)
		}
	}
	return instance, nil
}

func (b *backendLinux) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	err := b.start()
	if err != nil {
		b.Close()
		return err
	}
	return nil
}

func (b *backendLinux) start() error {
	if b.platform != nil {
		return b.startPlatform()
	}
	b.tunName = tun.CalculateInterfaceName(b.bridgeName)
	b.nftTableName = "sing-box-" + b.tunName
	tunInterface, err := tun.New(tun.Options{
		Name:                      b.tunName,
		MTU:                       bridgeTunMTU,
		GSO:                       true,
		InterfaceMonitor:          b.networkManager.InterfaceMonitor(),
		Logger:                    b.logger,
		EXP_ExternalConfiguration: true,
	})
	if err != nil {
		return E.Cause(err, "create bridge tun")
	}
	b.tunInterface = tunInterface
	err = tunInterface.Start()
	if err != nil {
		return E.Cause(err, "start bridge tun")
	}
	linuxTUN := tunInterface.(tun.LinuxTUN)
	if linuxTUN.BatchSize() > 1 {
		b.batchTUN = linuxTUN
		b.writeHeadroom = linuxTUN.FrontHeadroom()
		b.writeBuffers = make([][]byte, bridgeWriteBatchSize)
		for i := range b.writeBuffers {
			// handleGRO coalesces same-flow packets by appending into the first
			// packet's buffer capacity, up to the 0xffff total length limit.
			b.writeBuffers[i] = make([]byte, b.writeHeadroom+maxPacketLength)
		}
	}
	inet6Active, err := setupBridgeNetfilter(b.logger, b.nftTableName, b.tunName, b.inet6Port.IsValid())
	if err != nil {
		return E.Cause(err, "set up bridge netfilter")
	}
	if !inet6Active {
		b.inet6Port = netip.Addr{}
	}
	b.forwardingRestore = enableBridgeForwarding(b.logger, b.tunName, b.inet4Port.IsValid(), b.inet6Port.IsValid())
	if b.boundInterface != "" {
		b.syncEgress()
	}
	err = setupBridgeFamily(b.tunName, b.ruleIndex, b.routeTable, unix.AF_INET, b.inet4Port)
	if err != nil {
		return E.Cause(err, "set up bridge routing")
	}
	err = setupBridgeFamily(b.tunName, b.ruleIndex, b.routeTable, unix.AF_INET6, b.inet6Port)
	if err != nil {
		b.logger.Debug(E.Cause(err, "IPv6 bridge routing unavailable, disabling IPv6 forwarding"))
		removeBridgeFamily(b.tunName, b.ruleIndex, b.routeTable, unix.AF_INET6, b.inet6Port)
		b.inet6Port = netip.Addr{}
	}
	b.closed = make(chan struct{})
	b.readDone = make(chan struct{})
	if b.batchTUN != nil {
		go b.batchReadLoop()
	} else {
		go b.readLoop()
	}
	egress := "auto"
	if b.boundInterface != "" {
		egress = b.boundInterface
		monitor := b.networkManager.NetworkMonitor()
		if monitor != nil {
			element := monitor.RegisterCallback(func() { b.syncEgress() })
			b.unregister = func() { monitor.UnregisterCallback(element) }
		} else {
			b.logger.Debug("network monitor unavailable, pinned egress will not track interface changes")
		}
		b.syncEgress()
	} else {
		monitor := b.networkManager.InterfaceMonitor()
		if monitor != nil {
			element := monitor.RegisterCallback(func(_ *control.Interface, _ int) { b.updateClamp() })
			b.unregister = func() { monitor.UnregisterCallback(element) }
		}
		b.updateClamp()
	}
	natMode := "masquerade"
	if fullConeSupported() {
		natMode = "full-cone NAT"
	}
	b.logger.Info("bridge started at ", b.tunName, " (", natMode, ", egress ", egress, ")")
	return nil
}

func (b *backendLinux) startPlatform() error {
	session, err := b.platform.CreateBridge(adapter.BridgeOptions{
		BridgeName: b.bridgeName,
		MTU:        bridgeTunMTU,
		Inet4Port:  b.inet4Port,
		Inet6Port:  b.inet6Port,
		RuleIndex:  b.ruleIndex,
		RouteTable: b.routeTable,
	})
	if err != nil {
		return E.Cause(err, "create bridge")
	}
	b.session = session
	b.tunName = session.Name()
	if !session.Inet6Active() {
		b.inet6Port = netip.Addr{}
	}
	tunInterface, err := tun.New(tun.Options{
		Name:           b.tunName,
		MTU:            bridgeTunMTU,
		GSO:            true,
		FileDescriptor: session.FileDescriptor(),
		Logger:         b.logger,
	})
	if err != nil {
		return E.Cause(err, "create bridge tun")
	}
	b.tunInterface = tunInterface
	err = tunInterface.Start()
	if err != nil {
		return E.Cause(err, "start bridge tun")
	}
	linuxTUN := tunInterface.(tun.LinuxTUN)
	if linuxTUN.BatchSize() > 1 {
		b.batchTUN = linuxTUN
		b.writeHeadroom = linuxTUN.FrontHeadroom()
		b.writeBuffers = make([][]byte, bridgeWriteBatchSize)
		for i := range b.writeBuffers {
			b.writeBuffers[i] = make([]byte, b.writeHeadroom+maxPacketLength)
		}
	}
	b.closed = make(chan struct{})
	b.readDone = make(chan struct{})
	if b.batchTUN != nil {
		go b.batchReadLoop()
	} else {
		go b.readLoop()
	}
	monitor := b.networkManager.InterfaceMonitor()
	if monitor != nil {
		element := monitor.RegisterCallback(func(_ *control.Interface, _ int) { b.syncSessionEgress() })
		b.unregister = func() { monitor.UnregisterCallback(element) }
	}
	b.syncSessionEgress()
	egress := "auto"
	if b.boundInterface != "" {
		egress = b.boundInterface
	}
	b.logger.Info("bridge started at ", b.tunName, " (platform, egress ", egress, ")")
	return nil
}

func (b *backendLinux) Close() error {
	b.closeOnce.Do(func() {
		if b.closed != nil {
			close(b.closed)
		}
		if b.unregister != nil {
			b.unregister()
		}
		if b.tunInterface != nil {
			b.tunInterface.Close()
		}
		if b.readDone != nil {
			<-b.readDone
		}
		if b.session != nil {
			_ = b.session.Close()
		} else {
			b.egressAccess.Lock()
			if b.tunName != "" {
				cleanupBridgeNetfilter(b.nftTableName)
				removeBridgeFamily(b.tunName, b.ruleIndex, b.routeTable, unix.AF_INET, b.inet4Port)
				removeBridgeFamily(b.tunName, b.ruleIndex, b.routeTable, unix.AF_INET6, b.inet6Port)
			}
			if b.routeTable != 0 {
				flushBridgeRouteTable(b.routeTable)
			}
			b.egressAccess.Unlock()
			restoreBridgeForwarding(b.forwardingRestore)
			b.forwardingRestore = nil
		}
		releaseBridgeIndex(b.index)
	})
	return nil
}

// Zero tells the dispatcher not to clamp the TCP MSS or fragment; the host kernel
// does both on the forwarding path instead (see setupBridgeClampRules).
func (b *backendLinux) PortMTU() uint32 {
	return 0
}

func (b *backendLinux) WritePackets(packets [][]byte) error {
	if b.batchTUN == nil {
		for _, packet := range packets {
			if len(packet) == 0 {
				continue
			}
			_, err := b.tunInterface.Write(packet)
			if err != nil {
				return err
			}
		}
		return nil
	}
	b.writeAccess.Lock()
	defer b.writeAccess.Unlock()
	for len(packets) > 0 {
		chunk := packets
		if len(chunk) > len(b.writeBuffers) {
			chunk = chunk[:len(b.writeBuffers)]
		}
		packets = packets[len(chunk):]
		batch := make([][]byte, 0, len(chunk))
		for i, packet := range chunk {
			if len(packet) == 0 || len(packet) > maxPacketLength {
				continue
			}
			buffer := b.writeBuffers[i][:b.writeHeadroom+len(packet)]
			copy(buffer[b.writeHeadroom:], packet)
			batch = append(batch, buffer)
		}
		if len(batch) == 0 {
			continue
		}
		_, err := b.batchTUN.BatchWrite(batch, b.writeHeadroom)
		if err != nil {
			return err
		}
	}
	return nil
}

// BatchRead completes any kernel-deferred checksums while splitting GRO frames
// (virtio NEEDS_CSUM), so unlike readLoop no checksum fix is needed here.
func (b *backendLinux) batchReadLoop() {
	defer close(b.readDone)
	batchSize := b.batchTUN.BatchSize()
	sizes := make([]int, batchSize)
	batch := make([][]byte, 0, batchSize)
	headroom := -1
	var buffers [][]byte
	for {
		b.returnAccess.Lock()
		returnPaths := b.returnPaths
		b.returnAccess.Unlock()
		pathHeadroom := 0
		if len(returnPaths) > 0 {
			pathHeadroom = returnPaths[0].ReturnHeadroom()
		}
		if pathHeadroom != headroom {
			headroom = pathHeadroom
			buffers = make([][]byte, batchSize)
			for i := range buffers {
				buffers[i] = make([]byte, headroom+bridgeTunMTU)
			}
		}
		n, err := b.batchTUN.BatchRead(buffers, headroom, sizes)
		if err != nil {
			select {
			case <-b.closed:
				return
			default:
			}
			if E.IsClosed(err) {
				return
			}
			b.logger.Debug(E.Cause(err, "bridge tun read"))
			continue
		}
		if n == 0 || len(returnPaths) == 0 {
			continue
		}
		batch = batch[:0]
		for i := range n {
			if sizes[i] == 0 {
				continue
			}
			batch = append(batch, buffers[i][:headroom+sizes[i]])
		}
		unconsumed := batch
		currentHeadroom := headroom
		for _, returnPath := range returnPaths {
			if len(unconsumed) == 0 {
				break
			}
			nextHeadroom := returnPath.ReturnHeadroom()
			if nextHeadroom != currentHeadroom {
				rebuffered := make([][]byte, 0, len(unconsumed))
				for _, packet := range unconsumed {
					payload := packet[currentHeadroom:]
					buffer := make([]byte, nextHeadroom+len(payload))
					copy(buffer[nextHeadroom:], payload)
					rebuffered = append(rebuffered, buffer)
				}
				unconsumed = rebuffered
				currentHeadroom = nextHeadroom
			}
			unconsumed = returnPath.ReturnPackets(unconsumed)
		}
	}
}

// The policy rules default to priority 100/101, ahead of sing-tun auto_route's rules,
// so forwarded packets egress the physical interface instead of looping back into
// a tun.
func (b *backendLinux) syncEgress() {
	b.egressAccess.Lock()
	defer b.egressAccess.Unlock()
	select {
	case <-b.closed:
		return
	default:
	}
	b.updateClampLocked()
	flushBridgeRouteTable(b.routeTable)
	link, err := netlink.LinkByName(b.boundInterface)
	if err != nil {
		for _, family := range activeBridgeFamilies(b.inet6Port) {
			blackholeBridgeDefault(b.routeTable, family)
		}
		b.logger.Debug("pinned egress ", b.boundInterface, " absent, dropping forwarded traffic")
		return
	}
	for _, family := range activeBridgeFamilies(b.inet6Port) {
		b.syncEgressFamily(family, link.Attrs().Index)
	}
}

func (b *backendLinux) syncEgressFamily(family int, linkIndex int) {
	connected, err := netlink.RouteListFiltered(family, &netlink.Route{
		LinkIndex: linkIndex,
		Table:     unix.RT_TABLE_MAIN,
	}, netlink.RT_FILTER_OIF|netlink.RT_FILTER_TABLE)
	if err == nil {
		for _, route := range connected {
			if route.Gw != nil || route.Dst == nil {
				continue
			}
			pinned := route
			pinned.Table = b.routeTable
			pinned.ILinkIndex = 0
			_ = netlink.RouteReplace(&pinned)
		}
	}
	resolved, err := netlink.RouteGetWithOptions(probeAddress(family), &netlink.RouteGetOptions{Oif: b.boundInterface})
	if err == nil && len(resolved) > 0 {
		defaultRoute := &netlink.Route{
			LinkIndex: linkIndex,
			Table:     b.routeTable,
			Dst:       defaultDestination(family),
		}
		if len(resolved[0].Gw) > 0 {
			defaultRoute.Gw = resolved[0].Gw
		}
		err = netlink.RouteReplace(defaultRoute)
		if err == nil {
			return
		}
	}
	blackholeBridgeDefault(b.routeTable, family)
}

func (b *backendLinux) updateClamp() {
	b.egressAccess.Lock()
	defer b.egressAccess.Unlock()
	select {
	case <-b.closed:
		return
	default:
	}
	b.updateClampLocked()
}

func (b *backendLinux) updateClampLocked() {
	mtu := bridgeTunMTU
	egress := b.resolveEgress()
	if egress != "" {
		mtu = b.egressMTU(egress)
	}
	if mtu == b.clampMTU {
		return
	}
	err := setupBridgeClamp(b.nftTableName, b.tunName, b.inet4Port, b.inet6Port, mtu)
	if err != nil {
		b.logger.Debug(E.Cause(err, "update bridge MSS clamp"))
		return
	}
	b.clampMTU = mtu
}

func (b *backendLinux) egressMTU(egress string) int {
	iface, err := b.networkManager.InterfaceFinder().ByName(egress)
	if err != nil || iface.MTU < 576 || iface.MTU > bridgeTunMTU {
		return bridgeTunMTU
	}
	return iface.MTU
}
