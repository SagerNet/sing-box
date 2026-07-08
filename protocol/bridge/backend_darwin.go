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
	"github.com/sagernet/sing-tun/gtcpip/header"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/service"
)

var (
	bridgeInet4LocalBase = netip.MustParseAddr("198.51.100.1")
	bridgeInet6LocalBase = netip.MustParseAddr("2001:db8:1::1")
)

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
	err := instance.init(ctx, logger, networkManager, tag, options)
	if err != nil {
		return nil, err
	}
	instance.inet4Local = addressAt(bridgeInet4LocalBase, instance.index)
	instance.inet6Local = addressAt(bridgeInet6LocalBase, instance.index)
	platformInterface := service.FromContext[adapter.PlatformInterface](ctx)
	if platformInterface != nil && platformInterface.UsePlatformBridge() {
		instance.platform = platformInterface
	}
	return instance, nil
}

func (b *backendDarwin) Start(stage adapter.StartStage) error {
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

func (b *backendDarwin) start() error {
	if b.platform != nil {
		return b.startPlatform()
	}
	b.tunName = tun.CalculateInterfaceName(b.bridgeName)
	b.anchorName = "com.apple/sing-box-" + b.tunName
	tunInterface, err := tun.New(tun.Options{
		Name:                      b.tunName,
		MTU:                       bridgeTunMTU,
		AutoRoute:                 false,
		InterfaceMonitor:          b.networkManager.InterfaceMonitor(),
		Logger:                    b.logger,
		EXP_ExternalConfiguration: true,
		EXP_MultiPendingPackets:   true,
		EXP_SendMsgX:              true,
	})
	if err != nil {
		return E.Cause(err, "create bridge tun")
	}
	b.tunInterface = tunInterface
	err = tunInterface.Start()
	if err != nil {
		return E.Cause(err, "start bridge tun")
	}
	b.forwardingRestore = enableDarwinForwarding(b.logger, b.inet4Port.IsValid(), b.inet6Port.IsValid())
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
	b.batchTUN = tunInterface.(tun.DarwinTUN)
	b.closed = make(chan struct{})
	b.readDone = make(chan struct{})
	b.registerMonitors(b.syncEgress)
	b.syncEgress()
	go b.batchReadLoop()
	b.logger.Info("bridge started at ", b.tunName, " (masquerade, egress ", b.egressLabel(), ")")
	return nil
}

func (b *backendDarwin) startPlatform() error {
	session, err := b.platform.CreateBridge(adapter.BridgeOptions{
		BridgeName: b.bridgeName,
		MTU:        bridgeTunMTU,
		Inet4Port:  b.inet4Port,
		Inet6Port:  b.inet6Port,
		Interface:  b.boundInterface,
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
		Name:                      b.tunName,
		MTU:                       bridgeTunMTU,
		FileDescriptor:            session.FileDescriptor(),
		Logger:                    b.logger,
		EXP_ExternalConfiguration: true,
		EXP_MultiPendingPackets:   true,
	})
	if err != nil {
		return E.Cause(err, "create bridge tun")
	}
	b.tunInterface = tunInterface
	err = tunInterface.Start()
	if err != nil {
		return E.Cause(err, "start bridge tun")
	}
	b.batchTUN = tunInterface.(tun.DarwinTUN)
	b.closed = make(chan struct{})
	b.readDone = make(chan struct{})
	b.registerMonitors(b.syncSessionEgress)
	b.syncSessionEgress()
	go b.batchReadLoop()
	b.logger.Info("bridge started at ", b.tunName, " (platform, egress ", b.egressLabel(), ")")
	return nil
}

func (b *backendDarwin) egressLabel() string {
	if b.boundInterface != "" {
		return b.boundInterface
	}
	return "auto"
}

func (b *backendDarwin) Close() error {
	b.closeOnce.Do(func() {
		if b.closed != nil {
			close(b.closed)
		}
		if b.unregister != nil {
			b.unregister()
		}
		if b.pfDevice != nil && b.anchorName != "" {
			b.egressAccess.Lock()
			_ = b.pfDevice.LoadAnchor(b.anchorName, nil)
			b.egressAccess.Unlock()
		}
		restoreDarwinForwarding(b.forwardingRestore)
		b.forwardingRestore = nil
		if b.pfDevice != nil {
			if b.pfToken != 0 {
				_ = b.pfDevice.StopReference(b.pfToken)
			}
			_ = b.pfDevice.Close()
		}
		if b.tunInterface != nil {
			b.tunInterface.Close()
		}
		if b.readDone != nil {
			<-b.readDone
		}
		if b.session != nil {
			_ = b.session.Close()
		}
		releaseBridgeIndex(b.index)
	})
	return nil
}

// Zero tells the dispatcher not to clamp the TCP MSS or fragment; pf and the
// host kernel do both on the forwarding path instead (see buildBridgeAnchorRules).
func (b *backendDarwin) PortMTU() uint32 {
	return 0
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
			if len(packet) == 0 || len(packet) > maxPacketLength {
				continue
			}
			ipVersion := header.IPVersion(packet)
			if ipVersion != header.IPv4Version && ipVersion != header.IPv6Version {
				continue
			}
			batch = append(batch, buf.As(packet))
		}
		if len(batch) == 0 {
			continue
		}
		err := b.batchTUN.BatchWrite(batch)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *backendDarwin) batchReadLoop() {
	defer close(b.readDone)
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
	var rules []pfAnchorRule
	if egress != "" {
		rules = buildBridgeAnchorRules(b.logger, b.tunName, egress, b.boundInterface, b.inet4Port, b.inet6Port)
	}
	if slices.Equal(rules, b.currentRules) {
		return
	}
	err := b.pfDevice.LoadAnchor(b.anchorName, rules)
	if err != nil {
		b.logger.Debug(E.Cause(err, "apply bridge egress ", egress))
		return
	}
	b.currentRules = rules
	if len(rules) == 0 {
		b.logger.Debug("bridge egress unavailable, dropping forwarded traffic")
	} else {
		b.logger.Debug("bridge egress ", egress)
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
