package device

import (
	"context"
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	N "github.com/sagernet/sing/common/network"
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
	FrontHeadroom() int
	NewOutboundQueue(handler func(packetBuffers []*buf.Buffer)) *tun.OutboundQueue
	Close() error
}

type Options struct {
	Context             context.Context
	Logger              logger.ContextLogger
	System              bool
	Handler             tun.Handler
	UDPTimeout          time.Duration
	ICMPTimeout         time.Duration
	UDPMapping          tun.NATMapping
	UDPFiltering        tun.NATFiltering
	UDPNATMax           uint32
	InterfaceFinder     control.InterfaceFinder
	MemoryPressure      func() tun.MemoryPressure
	Name                string
	NamePrefix          string
	MTU                 uint32
	PacketFrontHeadroom int
	PacketRearHeadroom  int
	Route               func(packet []byte) *tun.OutboundQueue
	Configuration       Configuration
}

type Configuration struct {
	MTU       uint32
	Address   []netip.Prefix
	BlockIPv6 bool
}

func New(options Options) (Device, error) {
	if !options.System {
		return newStackDevice(options)
	}
	return newSystemStackDevice(options)
}

func newStack(options Options, memoryTun *tun.MemoryTun) (*tun.Go, error) {
	return tun.NewGo(tun.StackOptions{
		Context:         options.Context,
		Tun:             memoryTun,
		TunOptions:      tun.Options{MTU: options.MTU},
		UDPTimeout:      options.UDPTimeout,
		ICMPTimeout:     options.ICMPTimeout,
		UDPMapping:      options.UDPMapping,
		UDPFiltering:    options.UDPFiltering,
		UDPNATMax:       options.UDPNATMax,
		Handler:         options.Handler,
		Logger:          options.Logger,
		InterfaceFinder: options.InterfaceFinder,
		MemoryPressure:  options.MemoryPressure,
	})
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
	var temporaryBuffers []*buf.Buffer
	defer func() {
		buf.ReleaseMulti(temporaryBuffers)
	}()
	for i, packetBuffer := range packetBuffers {
		if packetBuffer.Start() < state.headroom {
			temporaryBuffer := buf.NewSize(state.headroom + packetBuffer.Len())
			temporaryBuffer.Resize(state.headroom, 0)
			common.Must1(temporaryBuffer.Write(packetBuffer.Bytes()))
			temporaryBuffers = append(temporaryBuffers, temporaryBuffer)
			packetBuffer = temporaryBuffer
		}
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
	newState := &returnPathState{
		returnPath: returnPath,
		headroom:   returnPath.ReturnHeadroom(),
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

func (d *baseDevice) FrontHeadroom() int {
	state := d.returnState.Load()
	if state == nil {
		return 0
	}
	return state.headroom
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
