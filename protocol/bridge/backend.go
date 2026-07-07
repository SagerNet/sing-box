//go:build linux || darwin

package bridge

import (
	"context"
	"net/netip"
	"slices"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
)

type sysctlState struct {
	name  string
	value string
}

type backendBase struct {
	ctx            context.Context
	logger         logger.ContextLogger
	networkManager adapter.NetworkManager
	tag            string

	index      uint32
	bridgeName string
	tunName    string
	inet4Port  netip.Addr
	inet6Port  netip.Addr

	boundInterface string

	tunInterface tun.Tun

	returnAccess sync.Mutex
	returnPaths  []tun.Return

	egressAccess      sync.Mutex
	forwardingRestore []sysctlState
	unregister        func()

	session       adapter.BridgeSession
	currentEgress string

	closeOnce sync.Once
	closed    chan struct{}
	readDone  chan struct{}
}

func (b *backendBase) init(ctx context.Context, logger logger.ContextLogger, networkManager adapter.NetworkManager, tag string, options option.BridgeOutboundOptions) error {
	index, err := allocateBridgeIndex()
	if err != nil {
		return err
	}
	b.ctx = ctx
	b.logger = logger
	b.networkManager = networkManager
	b.tag = tag
	b.index = index
	b.bridgeName = options.BridgeName
	if b.bridgeName == "" {
		b.bridgeName = "bridge"
	}
	b.boundInterface = options.Interface
	b.inet4Port = addressAt(bridgeInet4Base, index)
	b.inet6Port = addressAt(bridgeInet6Base, index)
	return nil
}

func (b *backendBase) PortAddresses() (netip.Addr, netip.Addr) {
	return b.inet4Port, b.inet6Port
}

func (b *backendBase) AttachReturn(returnPath tun.Return) error {
	b.returnAccess.Lock()
	defer b.returnAccess.Unlock()
	if slices.Contains(b.returnPaths, returnPath) {
		return nil
	}
	b.returnPaths = append(b.returnPaths[:len(b.returnPaths):len(b.returnPaths)], returnPath)
	return nil
}

func (b *backendBase) DetachReturn(returnPath tun.Return) error {
	b.returnAccess.Lock()
	defer b.returnAccess.Unlock()
	returnPaths := make([]tun.Return, 0, len(b.returnPaths))
	for _, existing := range b.returnPaths {
		if existing != returnPath {
			returnPaths = append(returnPaths, existing)
		}
	}
	b.returnPaths = returnPaths
	return nil
}

func (b *backendBase) syncSessionEgress() {
	b.egressAccess.Lock()
	defer b.egressAccess.Unlock()
	select {
	case <-b.closed:
		return
	default:
	}
	egress := b.resolveEgress()
	if egress == b.currentEgress {
		return
	}
	err := b.session.SetEgress(egress)
	if err != nil {
		b.logger.Debug(E.Cause(err, "apply bridge egress ", egress))
		return
	}
	b.currentEgress = egress
	if egress == "" {
		b.logger.Debug("bridge egress unavailable, dropping forwarded traffic")
	} else {
		b.logger.Debug("bridge egress ", egress)
	}
}

func (b *backendBase) resolveEgress() string {
	if b.boundInterface != "" {
		return b.boundInterface
	}
	monitor := b.networkManager.InterfaceMonitor()
	if monitor == nil {
		return ""
	}
	defaultInterface := monitor.DefaultInterface()
	if defaultInterface == nil {
		return ""
	}
	return defaultInterface.Name
}

func (b *backendBase) readLoop() {
	defer close(b.readDone)
	buffer := make([]byte, tun.PacketOffset+bridgeTunMTU)
	for {
		n, err := b.tunInterface.Read(buffer)
		if err != nil {
			select {
			case <-b.closed:
			default:
				b.logger.Debug(E.Cause(err, "bridge tun read"))
			}
			return
		}
		if n <= tun.PacketOffset {
			continue
		}
		packet := buffer[tun.PacketOffset:n]
		// On checksum-offloading NICs (notably virtio) the kernel leaves the L4
		// checksum uncomputed when the forwarding path TXes to a tun; recompute it.
		fixReturnChecksum(packet)
		b.deliverReturn(packet)
	}
}

func (b *backendBase) deliverReturn(packet []byte) {
	b.returnAccess.Lock()
	returnPaths := b.returnPaths
	b.returnAccess.Unlock()
	for _, returnPath := range returnPaths {
		headroom := returnPath.ReturnHeadroom()
		buffer := make([]byte, headroom+len(packet))
		copy(buffer[headroom:], packet)
		unconsumed := returnPath.ReturnPackets([][]byte{buffer})
		if len(unconsumed) == 0 {
			return
		}
	}
}
