//go:build linux || darwin

package bridge

import (
	"net/netip"
	"sync"

	"github.com/sagernet/sing-tun/gtcpip"
	"github.com/sagernet/sing-tun/gtcpip/checksum"
	"github.com/sagernet/sing-tun/gtcpip/header"
	E "github.com/sagernet/sing/common/exceptions"
)

const (
	bridgeTunMTU         = 1500
	maxPacketLength      = 0xffff
	bridgeMaxInstances   = 254
	bridgeWriteBatchSize = 32
)

var (
	bridgeInet4Base = netip.MustParseAddr("192.0.2.1")
	bridgeInet6Base = netip.MustParseAddr("2001:db8::1")

	bridgeIndexAccess sync.Mutex
	bridgeIndexInUse  [bridgeMaxInstances]bool
)

func allocateBridgeIndex() (uint32, error) {
	bridgeIndexAccess.Lock()
	defer bridgeIndexAccess.Unlock()
	for index := range bridgeMaxInstances {
		if !bridgeIndexInUse[index] {
			bridgeIndexInUse[index] = true
			return uint32(index), nil
		}
	}
	return 0, E.New("too many bridge outbounds: limit is ", bridgeMaxInstances)
}

func releaseBridgeIndex(index uint32) {
	bridgeIndexAccess.Lock()
	defer bridgeIndexAccess.Unlock()
	bridgeIndexInUse[index] = false
}

func addressAt(base netip.Addr, offset uint32) netip.Addr {
	addr := base
	for range offset {
		addr = addr.Next()
	}
	return addr
}

func fixReturnChecksum(packet []byte) {
	switch header.IPVersion(packet) {
	case header.IPv4Version:
		if len(packet) < header.IPv4MinimumSize {
			return
		}
		ipHdr := header.IPv4(packet)
		if !ipHdr.IsValid(len(packet)) {
			return
		}
		if ipHdr.Flags()&header.IPv4FlagMoreFragments != 0 || ipHdr.FragmentOffset() != 0 {
			return
		}
		ipHdr.SetChecksum(0)
		ipHdr.SetChecksum(^ipHdr.CalculateChecksum())
		recomputeTransportChecksum(ipHdr.TransportProtocol(), ipHdr.Payload(), ipHdr.SourceAddressSlice(), ipHdr.DestinationAddressSlice())
	case header.IPv6Version:
		if len(packet) < header.IPv6MinimumSize {
			return
		}
		ipHdr := header.IPv6(packet)
		recomputeTransportChecksum(ipHdr.TransportProtocol(), ipHdr.Payload(), ipHdr.SourceAddressSlice(), ipHdr.DestinationAddressSlice())
	}
}

func recomputeTransportChecksum(protocol tcpip.TransportProtocolNumber, transport []byte, source []byte, destination []byte) {
	switch protocol {
	case header.TCPProtocolNumber:
		if len(transport) < header.TCPMinimumSize {
			return
		}
		tcpHdr := header.TCP(transport)
		tcpHdr.SetChecksum(0)
		payloadChecksum := checksum.Checksum(tcpHdr.Payload(), 0)
		pseudoChecksum := header.PseudoHeaderChecksum(header.TCPProtocolNumber, source, destination, uint16(len(transport)))
		tcpHdr.SetChecksum(^tcpHdr.CalculateChecksum(checksum.Combine(pseudoChecksum, payloadChecksum)))
	case header.UDPProtocolNumber:
		if len(transport) < header.UDPMinimumSize {
			return
		}
		udpHdr := header.UDP(transport)
		udpHdr.SetChecksum(0)
		payloadChecksum := checksum.Checksum(udpHdr.Payload(), 0)
		pseudoChecksum := header.PseudoHeaderChecksum(header.UDPProtocolNumber, source, destination, udpHdr.Length())
		udpChecksum := ^udpHdr.CalculateChecksum(checksum.Combine(pseudoChecksum, payloadChecksum))
		if udpChecksum == 0 {
			udpChecksum = 0xffff
		}
		udpHdr.SetChecksum(udpChecksum)
	case header.ICMPv4ProtocolNumber:
		if len(transport) < header.ICMPv4MinimumSize {
			return
		}
		icmpHdr := header.ICMPv4(transport)
		icmpHdr.SetChecksum(header.ICMPv4Checksum(icmpHdr, 0))
	case header.ICMPv6ProtocolNumber:
		if len(transport) < header.ICMPv6MinimumSize {
			return
		}
		icmpHdr := header.ICMPv6(transport)
		icmpHdr.SetChecksum(header.ICMPv6Checksum(header.ICMPv6ChecksumParams{
			Header: icmpHdr,
			Src:    source,
			Dst:    destination,
		}))
	}
}
