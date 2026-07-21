package openconnect

import (
	"context"
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-openconnect"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	N "github.com/sagernet/sing/common/network"
)

const (
	DefaultMTU     = 1500
	PacketHeadroom = openconnect.PacketHeadroom
)

type PacketWriter func(packetBuffers []*buf.Buffer) error

type Device interface {
	N.Dialer
	Start() error
	UpdateConfiguration(configuration Configuration) error
	WriteInboundBuffers(packetBuffers []*buf.Buffer) error
	SetPacketWriter(writer PacketWriter)
	PortAddresses() (netip.Addr, netip.Addr)
	PortMTU() uint32
	AttachReturn(returnPath tun.Return) error
	DetachReturn(returnPath tun.Return) error
	ReturnPath() (tun.Return, int)
	Close() error
}

type DeviceOptions struct {
	Context          context.Context
	Logger           logger.ContextLogger
	System           bool
	Handler          tun.Handler
	UDPTimeout       time.Duration
	ICMPTimeout      time.Duration
	UDPMapping       tun.NATMapping
	UDPFiltering     tun.NATFiltering
	UDPNATMax        uint32
	InterfaceFinder  control.InterfaceFinder
	ExcludeInterface []string
	Name             string
	MTU              uint32
	Configuration    Configuration
}

type Configuration struct {
	MTU                      uint32
	Addresses                []netip.Prefix
	Routes                   []Route
	ExcludedRoutes           []Route
	DNS                      []netip.Addr
	NBNS                     []netip.Addr
	SearchDomains            []string
	SplitDNS                 []string
	SplitDNSRules            []SplitDNSRule
	ProxyAutoConfigURL       string
	Banner                   string
	TunnelAllDNS             bool
	ClientBypassProtocol     bool
	IdleTimeout              time.Duration
	AuthenticationExpiration time.Time
}

type Route struct {
	Prefix  netip.Prefix
	Gateway netip.Addr
	Metric  int
}

type SplitDNSRule struct {
	Domains []string
	Servers []netip.Addr
}

func NewDevice(options DeviceOptions) (Device, error) {
	if !options.System {
		return newStackDevice(options)
	}
	if !tun.WithGVisor {
		return newSystemDevice(options)
	}
	return newSystemStackDevice(options)
}

type baseDevice struct {
	packetWriter PacketWriter
	returnState  atomic.Pointer[returnPathState]
}

func (d *baseDevice) SetPacketWriter(writer PacketWriter) {
	d.packetWriter = writer
}

func (d *baseDevice) writeOutbound(packetBuffers []*buf.Buffer) error {
	if d.packetWriter == nil {
		buf.ReleaseMulti(packetBuffers)
		return E.New("missing packet writer")
	}
	return d.packetWriter(packetBuffers)
}

func (d *baseDevice) processInboundBuffers(packetBuffers []*buf.Buffer, writeBuffers func(packetBuffers []*buf.Buffer) error) error {
	if len(packetBuffers) == 0 {
		return nil
	}
	state := d.returnState.Load()
	if state == nil {
		return writeBuffers(packetBuffers)
	}
	packets := make([][]byte, len(packetBuffers))
	for i, packetBuffer := range packetBuffers {
		packetBuffer.ExtendHeader(state.headroom)
		packets[i] = packetBuffer.Bytes()
	}
	unconsumed := state.returnPath.ReturnPackets(packets)
	if len(unconsumed) == 0 {
		return nil
	}
	unconsumedBuffers := make([]*buf.Buffer, len(unconsumed))
	for i, packet := range unconsumed {
		packetBuffer := buf.As(packet)
		packetBuffer.Advance(state.headroom)
		unconsumedBuffers[i] = packetBuffer
	}
	return writeBuffers(unconsumedBuffers)
}

func (d *baseDevice) AttachReturn(returnPath tun.Return) error {
	headroom := returnPath.ReturnHeadroom()
	if headroom > PacketHeadroom {
		return E.New("return path headroom ", headroom, " exceeds available ", PacketHeadroom)
	}
	newState := &returnPathState{
		returnPath: returnPath,
		headroom:   headroom,
	}
	for {
		currentState := d.returnState.Load()
		if currentState != nil {
			if currentState.returnPath == returnPath {
				return nil
			}
			return E.New("return path already attached")
		}
		if d.returnState.CompareAndSwap(nil, newState) {
			return nil
		}
	}
}

func (d *baseDevice) DetachReturn(returnPath tun.Return) error {
	currentState := d.returnState.Load()
	if currentState != nil && currentState.returnPath == returnPath {
		d.returnState.CompareAndSwap(currentState, nil)
	}
	return nil
}

func (d *baseDevice) ReturnPath() (tun.Return, int) {
	state := d.returnState.Load()
	if state == nil {
		return nil, 0
	}
	return state.returnPath, state.headroom
}

type returnPathState struct {
	returnPath tun.Return
	headroom   int
}

func firstAddresses(addresses []netip.Prefix) (netip.Addr, netip.Addr) {
	var inet4Address netip.Addr
	var inet6Address netip.Addr
	for _, prefix := range addresses {
		if prefix.Addr().Is4() && !inet4Address.IsValid() {
			inet4Address = prefix.Addr()
		} else if prefix.Addr().Is6() && !inet6Address.IsValid() {
			inet6Address = prefix.Addr()
		}
	}
	return inet4Address, inet6Address
}

func splitPrefixes(prefixes []netip.Prefix) ([]netip.Prefix, []netip.Prefix) {
	var inet4Prefixes []netip.Prefix
	var inet6Prefixes []netip.Prefix
	for _, prefix := range prefixes {
		if prefix.Addr().Is4() {
			inet4Prefixes = append(inet4Prefixes, prefix)
		} else {
			inet6Prefixes = append(inet6Prefixes, prefix)
		}
	}
	return inet4Prefixes, inet6Prefixes
}
