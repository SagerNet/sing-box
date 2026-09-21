package masque

import (
	"net/netip"

	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing-tun/gtcpip/header"
	"github.com/sagernet/sing/common/buf"
)

func packetAddresses(packet []byte) (netip.Addr, netip.Addr, uint8, bool) {
	protocol, valid := tun.IPTransportProtocol(packet)
	if !valid {
		return netip.Addr{}, netip.Addr{}, 0, false
	}
	if header.IPVersion(packet) == header.IPv4Version {
		ipHeader := header.IPv4(packet)
		return ipHeader.SourceAddr(), ipHeader.DestinationAddr(), protocol, true
	}
	ipHeader := header.IPv6(packet)
	return ipHeader.SourceAddr(), ipHeader.DestinationAddr(), protocol, true
}

func isControlProtocol(protocol uint8) bool {
	return protocol == uint8(header.ICMPv4ProtocolNumber) || protocol == uint8(header.ICMPv6ProtocolNumber)
}

func decrementHopLimit(packet []byte) bool {
	if header.IPVersion(packet) == header.IPv4Version {
		ipHeader := header.IPv4(packet)
		if ipHeader.TTL() <= 1 {
			return false
		}
		ipHeader.SetTTL(ipHeader.TTL() - 1)
		ipHeader.SetChecksum(0)
		ipHeader.SetChecksum(^ipHeader.CalculateChecksum())
		return true
	}
	ipHeader := header.IPv6(packet)
	if ipHeader.HopLimit() <= 1 {
		return false
	}
	ipHeader.SetHopLimit(ipHeader.HopLimit() - 1)
	return true
}

func buildICMPError(packet []byte, errorType tun.ICMPError, inet4Source netip.Addr, inet6Source netip.Addr, mtu int, headroom int) (*buf.Buffer, bool) {
	source := inet4Source
	if header.IPVersion(packet) == header.IPv6Version {
		source = inet6Source
	}
	reply, built := tun.BuildICMPError(packet, errorType, source, uint32(mtu), headroom)
	if !built {
		return nil, false
	}
	buffer := buf.As(reply)
	buffer.Advance(headroom)
	return buffer, true
}
