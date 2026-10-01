package bridge

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"sync"
	"syscall"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/service"
)

var (
	bridgeInet4LocalBase = netip.MustParseAddr("198.51.100.1")
	bridgeInet6LocalBase = netip.MustParseAddr("2001:db8:1::1")
)

// macOS 26.5 (xnu-12377) kernel-panics ("Bounds safety trap") when pf
// route-to hands an unfragmented packet larger than one skywalk buflet to a
// skywalk-native interface: nx_netif_mbuf_to_kpkt() sizes the allocation
// against the TX pool, but nx_netif.c selects the copy routine from the RX
// pool's pp_max_frags, so pkt_copy_from_mbuf() writes past the 2048-byte
// buflet. utun_ctl_send() accepts writes of any size regardless of the
// interface MTU, so the limit must hold before packets are written; utun
// reserves UTUN_IF_HEADROOM_SIZE (32) bytes of the buflet, hence 2048-32.
const bridgeTunMTUDarwin = 2048 - 32

type backendDarwin struct {
	backendBase

	// anchorName lives under com.apple/* so the stock pf.conf's wildcard
	// nat/scrub/anchor references evaluate our rules without editing it.
	anchorName string

	inet4Local netip.Addr
	inet6Local netip.Addr

	batchTUN tun.DarwinTUN

	writeAccess sync.Mutex
	writeBatch  []*buf.Buffer

	pfDevice *pfDevice
	pfToken  uint64

	currentRules []pfAnchorRule

	platform adapter.PlatformInterface
}

func newBackend(ctx context.Context, logger logger.ContextLogger, networkManager adapter.NetworkManager, tag string, options option.BridgeOutboundOptions) (Backend, error) {
	instance := &backendDarwin{
		writeBatch: make([]*buf.Buffer, 0, bridgeWriteBatchSize),
	}
	instance.init(ctx, logger, networkManager, tag, options)
	platformInterface := service.FromContext[adapter.PlatformInterface](ctx)
	if platformInterface != nil && platformInterface.UsePlatformBridge() {
		instance.platform = platformInterface
	}
	return instance, nil
}

func (b *backendDarwin) Start(stage adapter.StartStage, scope *adapter.Scope) error {
	switch stage {
	case adapter.StartStateInitialize:
		err := b.allocateIndex(scope)
		if err != nil {
			return err
		}
		b.inet4Local = addressAt(bridgeInet4LocalBase, b.index)
		b.inet6Local = addressAt(bridgeInet6LocalBase, b.index)
	case adapter.StartStateStart:
		b.closed = scope.Context().Done()
		if b.platform != nil {
			return b.startPlatform(scope)
		}
		return b.start(scope)
	}
	return nil
}

func (b *backendDarwin) start(scope *adapter.Scope) error {
	b.tunName = tun.CalculateInterfaceName(b.bridgeName)
	b.anchorName = "com.apple/sing-box-" + b.tunName
	scope.Add(func() error {
		b.readGroup.Wait()
		return nil
	})
	tunInterface, err := tun.New(tun.Options{
		Name:                      b.tunName,
		MTU:                       bridgeTunMTUDarwin,
		AutoRoute:                 false,
		InterfaceMonitor:          b.networkManager.InterfaceMonitor(),
		Logger:                    b.logger,
		EXP_ExternalConfiguration: true,
		EXP_MultiPendingPackets:   true,
	})
	if err != nil {
		return E.Cause(err, "create bridge tun")
	}
	scope.Add(tunInterface.Close)
	b.tunInterface = tunInterface
	err = tunInterface.Start()
	if err != nil {
		return E.Cause(err, "start bridge tun")
	}
	forwardingRestore := enableDarwinForwarding(b.logger, b.inet4Port.IsValid(), b.inet6Port.IsValid())
	scope.Add(func() error {
		restoreDarwinForwarding(forwardingRestore)
		return nil
	})
	err = assignBridgePortAddress(b.tunName, b.inet4Local, b.inet4Port)
	if err != nil {
		return E.Cause(err, "add bridge route")
	}
	err = assignBridgePortAddress(b.tunName, b.inet6Local, b.inet6Port)
	if err != nil {
		b.logger.Debug(E.Cause(err, "IPv6 bridge routing unavailable, disabling IPv6 forwarding"))
		b.inet6Port = netip.Addr{}
	}
	err = b.enablePf()
	if err != nil {
		return E.Cause(err, "enable pf")
	}
	scope.Add(func() error {
		_ = b.pfDevice.StopReference(b.pfToken)
		return b.pfDevice.Close()
	})
	dropRules := bridgeDropRules(b.tunName, b.inet4Port, b.inet6Port)
	err = b.pfDevice.LoadAnchor(b.anchorName, dropRules)
	if err != nil {
		return E.Cause(err, "initialize bridge pf rules")
	}
	scope.Add(func() error {
		b.egressAccess.Lock()
		_ = b.pfDevice.LoadAnchor(b.anchorName, nil)
		b.egressAccess.Unlock()
		return nil
	})
	b.currentRules = dropRules
	b.batchTUN = tunInterface.(tun.DarwinTUN)
	b.registerMonitors(scope, b.syncEgress)
	b.syncEgress()
	b.readGroup.Go(b.batchReadLoop)
	b.logger.Info("bridge started at ", b.tunName, " (masquerade, egress ", b.egressLabel(), ")")
	return nil
}

func (b *backendDarwin) startPlatform(scope *adapter.Scope) error {
	session, err := b.platform.CreateBridge(adapter.BridgeOptions{
		BridgeName: b.bridgeName,
		MTU:        bridgeTunMTUDarwin,
		Inet4Port:  b.inet4Port,
		Inet6Port:  b.inet6Port,
		Interface:  b.boundInterface,
	})
	if err != nil {
		return E.Cause(err, "create bridge")
	}
	scope.Add(session.Close)
	b.session = session
	b.tunName = session.Name()
	if !session.Inet6Active() {
		b.inet6Port = netip.Addr{}
	}
	scope.Add(func() error {
		b.readGroup.Wait()
		return nil
	})
	tunInterface, err := tun.New(tun.Options{
		Name:                      b.tunName,
		MTU:                       bridgeTunMTUDarwin,
		FileDescriptor:            session.FileDescriptor(),
		Logger:                    b.logger,
		EXP_ExternalConfiguration: true,
		EXP_MultiPendingPackets:   true,
	})
	if err != nil {
		return E.Cause(err, "create bridge tun")
	}
	scope.Add(tunInterface.Close)
	b.tunInterface = tunInterface
	err = tunInterface.Start()
	if err != nil {
		return E.Cause(err, "start bridge tun")
	}
	b.batchTUN = tunInterface.(tun.DarwinTUN)
	b.registerMonitors(scope, b.syncSessionEgress)
	b.syncSessionEgress()
	b.readGroup.Go(b.batchReadLoop)
	b.logger.Info("bridge started at ", b.tunName, " (platform, egress ", b.egressLabel(), ")")
	return nil
}

func (b *backendDarwin) egressLabel() string {
	if b.boundInterface != "" {
		return b.boundInterface
	}
	return "auto"
}

func (b *backendDarwin) PortMTU() uint32 {
	return bridgeTunMTUDarwin
}

func (b *backendDarwin) WritePackets(packets [][]byte) error {
	b.writeAccess.Lock()
	defer b.writeAccess.Unlock()
	for len(packets) > 0 {
		chunk := packets
		if len(chunk) > bridgeWriteBatchSize {
			chunk = chunk[:bridgeWriteBatchSize]
		}
		packets = packets[len(chunk):]
		batch := b.writeBatch[:0]
		for _, packet := range chunk {
			batch = append(batch, buf.As(packet))
		}
		err := b.batchTUN.BatchWrite(batch)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *backendDarwin) batchReadLoop() {
	headroom := -1
	var buffers [][]byte
	var batch [][]byte
	for {
		packets, err := b.batchTUN.BatchRead()
		if err != nil {
			select {
			case <-b.closed:
				return
			default:
			}
			if E.IsClosed(err) || errors.Is(err, syscall.EBADF) {
				return
			}
			b.logger.Debug(E.Cause(err, "bridge tun read"))
			continue
		}
		if len(packets) == 0 {
			continue
		}
		b.returnAccess.Lock()
		returnPaths := b.returnPaths
		b.returnAccess.Unlock()
		if len(returnPaths) == 0 {
			buf.ReleaseMulti(packets)
			continue
		}
		pathHeadroom := returnPaths[0].ReturnHeadroom()
		if pathHeadroom != headroom {
			headroom = pathHeadroom
			buffers = buffers[:0]
		}
		for len(buffers) < len(packets) {
			buffers = append(buffers, make([]byte, headroom+bridgeTunMTU))
		}
		batch = batch[:0]
		for _, packet := range packets {
			payload := packet.Bytes()
			if len(payload) == 0 {
				continue
			}
			fixReturnChecksum(payload)
			buffer := buffers[len(batch)][:headroom+len(payload)]
			copy(buffer[headroom:], payload)
			batch = append(batch, buffer)
		}
		buf.ReleaseMulti(packets)
		if len(batch) == 0 {
			continue
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

func (b *backendDarwin) syncEgress() {
	b.egressAccess.Lock()
	defer b.egressAccess.Unlock()
	select {
	case <-b.closed:
		return
	default:
	}
	egress := b.resolveEgress()
	rules := bridgeDropRules(b.tunName, b.inet4Port, b.inet6Port)
	var buildErr error
	if egress != "" {
		rules, buildErr = buildBridgeAnchorRules(b.tunName, egress, b.boundInterface, b.inet4Port, b.inet6Port)
	}
	if slices.Equal(rules, b.currentRules) {
		if buildErr != nil {
			b.logger.Debug(buildErr)
		}
		return
	}
	err := b.pfDevice.LoadAnchor(b.anchorName, rules)
	if err != nil {
		b.logger.Debug(E.Cause(err, "apply bridge egress ", egress))
		return
	}
	b.currentRules = rules
	if buildErr != nil || egress == "" {
		b.logger.Debug("bridge egress unavailable, dropping forwarded traffic")
	} else {
		b.logger.Debug("bridge egress ", egress)
	}
	if buildErr != nil {
		b.logger.Debug(buildErr)
	}
}

func (b *backendDarwin) enablePf() error {
	device, err := openPfDevice()
	if err != nil {
		return err
	}
	token, err := device.StartReference()
	if err != nil {
		_ = device.Close()
		return err
	}
	b.pfDevice = device
	b.pfToken = token
	return nil
}
