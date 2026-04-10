//go:build linux && with_gvisor

package xdp

import (
	"encoding/binary"
	"net"
	"net/netip"
	"sync"

	"github.com/sagernet/gvisor/pkg/buffer"
	"github.com/sagernet/gvisor/pkg/tcpip"
	"github.com/sagernet/gvisor/pkg/tcpip/header"
	"github.com/sagernet/gvisor/pkg/tcpip/stack"
)

var _ stack.LinkEndpoint = (*xdpEndpoint)(nil)

var broadcastMAC = []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

// xdpEndpoint implements gVisor's stack.LinkEndpoint backed by AF_XDP.
// It strips/adds ethernet headers to convert between L2 (AF_XDP) and L3 (gVisor).
type xdpEndpoint struct {
	mu         sync.RWMutex
	mtu        uint32
	xsks       []*XSKSocket
	dispatcher stack.NetworkDispatcher
	done       chan struct{}
	localMAC   net.HardwareAddr // NIC's real MAC address for TX source
	macCache   sync.Map         // netip.Addr → [6]byte, learned from incoming packets
}

func newXDPEndpoint(xsks []*XSKSocket, mtu uint32, localMAC net.HardwareAddr) *xdpEndpoint {
	return &xdpEndpoint{
		xsks:     xsks,
		mtu:      mtu,
		done:     make(chan struct{}),
		localMAC: localMAC,
	}
}

func (ep *xdpEndpoint) MTU() uint32 {
	return ep.mtu
}

func (ep *xdpEndpoint) SetMTU(mtu uint32) {
	ep.mtu = mtu
}

func (ep *xdpEndpoint) MaxHeaderLength() uint16 {
	return 0
}

func (ep *xdpEndpoint) LinkAddress() tcpip.LinkAddress {
	return ""
}

func (ep *xdpEndpoint) SetLinkAddress(addr tcpip.LinkAddress) {
}

func (ep *xdpEndpoint) Capabilities() stack.LinkEndpointCapabilities {
	return stack.CapabilityRXChecksumOffload
}

func (ep *xdpEndpoint) Attach(dispatcher stack.NetworkDispatcher) {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	ep.dispatcher = dispatcher
	if dispatcher != nil {
		for _, xsk := range ep.xsks {
			go ep.rxLoop(xsk)
		}
	}
}

func (ep *xdpEndpoint) IsAttached() bool {
	ep.mu.RLock()
	defer ep.mu.RUnlock()
	return ep.dispatcher != nil
}

func (ep *xdpEndpoint) Wait() {
}

func (ep *xdpEndpoint) ARPHardwareType() header.ARPHardwareType {
	return header.ARPHardwareNone
}

func (ep *xdpEndpoint) AddHeader(pkt *stack.PacketBuffer) {
}

func (ep *xdpEndpoint) ParseHeader(pkt *stack.PacketBuffer) bool {
	return true
}

// WritePackets receives IP packets from gVisor and sends them via AF_XDP.
// gVisor gives us L3 (IP) packets; we prepend an ethernet header for AF_XDP.
// Uses the first available XSK socket for transmission.
func (ep *xdpEndpoint) WritePackets(list stack.PacketBufferList) (int, tcpip.Error) {
	if len(ep.xsks) == 0 {
		return 0, &tcpip.ErrClosedForSend{}
	}
	xsk := ep.xsks[0]

	var count int
	for _, pkt := range list.AsSlice() {
		data := pkt.AsSlices()
		totalLen := 0
		for _, s := range data {
			totalLen += len(s)
		}

		// Build ethernet frame: dst(6) + src(6) + ethertype(2) + payload
		frame := make([]byte, ethHeaderLen+totalLen)

		// Copy IP payload first so we can inspect the destination IP
		offset := ethHeaderLen
		for _, s := range data {
			copy(frame[offset:], s)
			offset += len(s)
		}

		// Look up destination MAC from cache; fall back to broadcast
		dstMAC := ep.lookupDstMAC(frame[ethHeaderLen:], totalLen)
		copy(frame[0:6], dstMAC)
		// Source MAC = NIC's real hardware address
		if len(ep.localMAC) >= 6 {
			copy(frame[6:12], ep.localMAC[:6])
		}

		if totalLen > 0 {
			switch header.IPVersion(frame[ethHeaderLen:]) {
			case header.IPv4Version:
				binary.BigEndian.PutUint16(frame[12:14], uint16(header.IPv4ProtocolNumber))
			case header.IPv6Version:
				binary.BigEndian.PutUint16(frame[12:14], uint16(header.IPv6ProtocolNumber))
			default:
				continue
			}
		}

		select {
		case <-ep.done:
			return count, &tcpip.ErrClosedForSend{}
		default:
		}

		if xsk.Transmit(frame) {
			count++
		}
	}

	if count > 0 {
		ep.xsks[0].FlushTX()
	}

	return count, nil
}

func (ep *xdpEndpoint) Close() {
	select {
	case <-ep.done:
	default:
		close(ep.done)
	}
}

func (ep *xdpEndpoint) SetOnCloseAction(f func()) {
}

func (ep *xdpEndpoint) lookupDstMAC(ipPayload []byte, payloadLen int) []byte {
	if payloadLen < 1 {
		return broadcastMAC
	}
	var dstAddr netip.Addr
	switch header.IPVersion(ipPayload) {
	case header.IPv4Version:
		if payloadLen < header.IPv4MinimumSize {
			return broadcastMAC
		}
		dstAddr = netip.AddrFrom4([4]byte(ipPayload[16:20]))
	case header.IPv6Version:
		if payloadLen < header.IPv6MinimumSize {
			return broadcastMAC
		}
		dstAddr = netip.AddrFrom16([16]byte(ipPayload[24:40]))
	default:
		return broadcastMAC
	}
	if val, ok := ep.macCache.Load(dstAddr); ok {
		mac := val.([6]byte)
		return mac[:]
	}
	return broadcastMAC
}

// learnMAC caches the source IP → source MAC mapping from an incoming frame.
func (ep *xdpEndpoint) learnMAC(frame []byte) {
	if len(frame) < ethHeaderLen+20 { // need at least eth + IPv4 minimum
		return
	}
	var srcMAC [6]byte
	copy(srcMAC[:], frame[6:12])
	// Skip zero or broadcast MACs
	if srcMAC == [6]byte{} || srcMAC == [6]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff} {
		return
	}

	etherType := binary.BigEndian.Uint16(frame[12:14])
	var srcAddr netip.Addr
	switch etherType {
	case 0x0800: // IPv4
		payload := frame[ethHeaderLen:]
		if len(payload) < header.IPv4MinimumSize {
			return
		}
		srcAddr = netip.AddrFrom4([4]byte(payload[12:16]))
	case 0x86DD: // IPv6
		payload := frame[ethHeaderLen:]
		if len(payload) < header.IPv6MinimumSize {
			return
		}
		srcAddr = netip.AddrFrom16([16]byte(payload[8:24]))
	case 0x8100: // VLAN
		if len(frame) < 18+20 {
			return
		}
		innerType := binary.BigEndian.Uint16(frame[16:18])
		payload := frame[18:]
		switch innerType {
		case 0x0800:
			if len(payload) < header.IPv4MinimumSize {
				return
			}
			srcAddr = netip.AddrFrom4([4]byte(payload[12:16]))
		case 0x86DD:
			if len(payload) < header.IPv6MinimumSize {
				return
			}
			srcAddr = netip.AddrFrom16([16]byte(payload[8:24]))
		default:
			return
		}
	default:
		return
	}
	if srcAddr.IsValid() && !srcAddr.IsUnspecified() {
		ep.macCache.Store(srcAddr, srcMAC)
	}
}

// rxLoop reads ethernet frames from a specific AF_XDP socket, strips the ethernet
// header, and delivers L3 (IP) packets to the gVisor network dispatcher.
func (ep *xdpEndpoint) rxLoop(xsk *XSKSocket) {
	for {
		select {
		case <-ep.done:
			return
		default:
		}

		// Poll for incoming packets
		if !xsk.Poll(100) {
			continue
		}

		frames, addrs := xsk.Receive()
		if len(frames) == 0 {
			continue
		}

		for _, frame := range frames {
			if len(frame) < ethHeaderLen {
				continue
			}

			// Learn source IP → MAC mapping from incoming frames
			ep.learnMAC(frame)

			// Parse ethernet header for ethertype
			etherType := binary.BigEndian.Uint16(frame[12:14])

			var networkProtocol tcpip.NetworkProtocolNumber
			switch etherType {
			case 0x0800: // IPv4
				networkProtocol = header.IPv4ProtocolNumber
			case 0x86DD: // IPv6
				networkProtocol = header.IPv6ProtocolNumber
			case 0x8100: // VLAN
				if len(frame) < ethHeaderLen+4 {
					continue
				}
				etherType = binary.BigEndian.Uint16(frame[16:18])
				switch etherType {
				case 0x0800:
					networkProtocol = header.IPv4ProtocolNumber
				case 0x86DD:
					networkProtocol = header.IPv6ProtocolNumber
				default:
					continue
				}
				// Strip ethernet + VLAN header (18 bytes)
				payload := frame[18:]
				pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{
					Payload: buffer.MakeWithData(payload),
				})
				ep.dispatcher.DeliverNetworkPacket(networkProtocol, pkt)
				pkt.DecRef()
				continue
			default:
				continue
			}

			// Strip ethernet header (14 bytes), pass IP payload to gVisor
			payload := frame[ethHeaderLen:]
			pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{
				Payload: buffer.MakeWithData(payload),
			})
			ep.dispatcher.DeliverNetworkPacket(networkProtocol, pkt)
			pkt.DecRef()
		}

		// Return frames to fill ring
		xsk.FreeRXFrames(addrs)
	}
}
