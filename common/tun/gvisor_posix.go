package tun

import (
	"os"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/rw"

	gBuffer "gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

var _ stack.LinkEndpoint = (*PosixEndpoint)(nil)

type PosixEndpoint struct {
	fd         uintptr
	mtu        uint32
	file       *os.File
	dispatcher stack.NetworkDispatcher
}

func NewPosixEndpoint(tunFd uintptr, tunMtu uint32) (stack.LinkEndpoint, error) {
	return &PosixEndpoint{
		fd:   tunFd,
		mtu:  tunMtu,
		file: os.NewFile(tunFd, "tun"),
	}, nil
}

func (e *PosixEndpoint) MTU() uint32 {
	return e.mtu
}

func (e *PosixEndpoint) MaxHeaderLength() uint16 {
	return 0
}

func (e *PosixEndpoint) LinkAddress() tcpip.LinkAddress {
	return ""
}

func (e *PosixEndpoint) Capabilities() stack.LinkEndpointCapabilities {
	return stack.CapabilityNone
}

func (e *PosixEndpoint) Attach(dispatcher stack.NetworkDispatcher) {
	if dispatcher == nil && e.dispatcher != nil {
		e.dispatcher = nil
		return
	}
	if dispatcher != nil && e.dispatcher == nil {
		e.dispatcher = dispatcher
		go e.dispatchLoop()
	}
}

func (e *PosixEndpoint) dispatchLoop() {
	_buffer := buf.StackNewPacket()
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	for {
		n, err := e.file.Read(buffer.FreeBytes())
		if err != nil {
			break
		}
		var view gBuffer.View
		view.Append(buffer.To(n))
		pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{
			Payload:           view,
			IsForwardedPacket: true,
		})
		defer pkt.DecRef()
		var p tcpip.NetworkProtocolNumber
		ipHeader, ok := pkt.Data().PullUp(1)
		if !ok {
			continue
		}
		switch header.IPVersion(ipHeader) {
		case header.IPv4Version:
			p = header.IPv4ProtocolNumber
		case header.IPv6Version:
			p = header.IPv6ProtocolNumber
		default:
			continue
		}
		e.dispatcher.DeliverNetworkPacket(p, pkt)
	}
}

func (e *PosixEndpoint) IsAttached() bool {
	return e.dispatcher != nil
}

func (e *PosixEndpoint) Wait() {
}

func (e *PosixEndpoint) ARPHardwareType() header.ARPHardwareType {
	return header.ARPHardwareNone
}

func (e *PosixEndpoint) AddHeader(buffer *stack.PacketBuffer) {
}

func (e *PosixEndpoint) WritePackets(pkts stack.PacketBufferList) (int, tcpip.Error) {
	var n int
	for _, packet := range pkts.AsSlice() {
		_, err := rw.WriteV(e.fd, packet.Slices())
		if err != nil {
			return n, &tcpip.ErrAborted{}
		}
		n++
	}
	return n, nil
}
