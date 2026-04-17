package tlsspoof

import (
	"net/netip"

	"github.com/sagernet/sing-tun/gtcpip/checksum"
	"github.com/sagernet/sing-tun/gtcpip/header"
	E "github.com/sagernet/sing/common/exceptions"
)

const (
	defaultTTL        uint8  = 64
	defaultWindowSize uint16 = 0xFFFF
	tcpHeaderLen             = header.TCPMinimumSize
)

func buildTCPSegment(
	src netip.AddrPort,
	dst netip.AddrPort,
	seqNum uint32,
	ackNum uint32,
	payload []byte,
	corruptChecksum bool,
) []byte {
	if src.Addr().Is4() != dst.Addr().Is4() {
		panic("tlsspoof: mixed IPv4/IPv6 address family")
	}
	var (
		frame       []byte
		ipHeaderLen int
	)
	if src.Addr().Is4() {
		ipHeaderLen = header.IPv4MinimumSize
		frame = make([]byte, ipHeaderLen+tcpHeaderLen+len(payload))
		ip := header.IPv4(frame[:ipHeaderLen])
		ip.Encode(&header.IPv4Fields{
			TotalLength: uint16(len(frame)),
			ID:          0,
			TTL:         defaultTTL,
			Protocol:    uint8(header.TCPProtocolNumber),
			SrcAddr:     src.Addr(),
			DstAddr:     dst.Addr(),
		})
		ip.SetChecksum(^ip.CalculateChecksum())
	} else {
		ipHeaderLen = header.IPv6MinimumSize
		frame = make([]byte, ipHeaderLen+tcpHeaderLen+len(payload))
		ip := header.IPv6(frame[:ipHeaderLen])
		ip.Encode(&header.IPv6Fields{
			PayloadLength:     uint16(tcpHeaderLen + len(payload)),
			TransportProtocol: header.TCPProtocolNumber,
			HopLimit:          defaultTTL,
			SrcAddr:           src.Addr(),
			DstAddr:           dst.Addr(),
		})
	}
	encodeTCP(frame, ipHeaderLen, src, dst, seqNum, ackNum, payload, corruptChecksum)
	return frame
}

func encodeTCP(frame []byte, ipHeaderLen int, src, dst netip.AddrPort, seqNum, ackNum uint32, payload []byte, corruptChecksum bool) {
	tcp := header.TCP(frame[ipHeaderLen:])
	copy(frame[ipHeaderLen+tcpHeaderLen:], payload)
	tcp.Encode(&header.TCPFields{
		SrcPort:    src.Port(),
		DstPort:    dst.Port(),
		SeqNum:     seqNum,
		AckNum:     ackNum,
		DataOffset: tcpHeaderLen,
		Flags:      header.TCPFlagAck | header.TCPFlagPsh,
		WindowSize: defaultWindowSize,
	})
	applyTCPChecksum(tcp, src.Addr(), dst.Addr(), payload, corruptChecksum)
}

func buildSpoofFrame(method Method, src, dst netip.AddrPort, sendNext, receiveNext uint32, payload []byte) ([]byte, error) {
	sequence, corrupt, err := resolveSpoofSequence(method, sendNext, payload)
	if err != nil {
		return nil, err
	}
	return buildTCPSegment(src, dst, sequence, receiveNext, payload, corrupt), nil
}

// buildSpoofTCPSegment returns a TCP segment without an IP header, for
// platforms where the kernel synthesises the IP header (darwin IPv6).
func buildSpoofTCPSegment(method Method, src, dst netip.AddrPort, sendNext, receiveNext uint32, payload []byte) ([]byte, error) {
	sequence, corrupt, err := resolveSpoofSequence(method, sendNext, payload)
	if err != nil {
		return nil, err
	}
	segment := make([]byte, tcpHeaderLen+len(payload))
	encodeTCP(segment, 0, src, dst, sequence, receiveNext, payload, corrupt)
	return segment, nil
}

func resolveSpoofSequence(method Method, sendNext uint32, payload []byte) (uint32, bool, error) {
	switch method {
	case MethodWrongSequence:
		return sendNext - uint32(len(payload)), false, nil
	case MethodWrongChecksum:
		return sendNext, true, nil
	default:
		return 0, false, E.New("tls_spoof: unknown method ", method)
	}
}

func applyTCPChecksum(tcp header.TCP, srcAddr, dstAddr netip.Addr, payload []byte, corrupt bool) {
	tcpLen := tcpHeaderLen + len(payload)
	pseudo := header.PseudoHeaderChecksum(header.TCPProtocolNumber, srcAddr.AsSlice(), dstAddr.AsSlice(), uint16(tcpLen))
	payloadChecksum := checksum.Checksum(payload, 0)
	tcpChecksum := ^tcp.CalculateChecksum(checksum.Combine(pseudo, payloadChecksum))
	if corrupt {
		tcpChecksum ^= 0xFFFF
	}
	tcp.SetChecksum(tcpChecksum)
}
