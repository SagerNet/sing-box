package tlsspoof

import (
	"encoding/binary"
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

type spoofPacketInfo struct {
	seqNum  uint32
	ackNum  uint32
	corrupt bool
	options []byte
}

func buildTCPSegment(
	src netip.AddrPort,
	dst netip.AddrPort,
	packetInfo spoofPacketInfo,
	payload []byte,
) []byte {
	if src.Addr().Is4() != dst.Addr().Is4() {
		panic("tlsspoof: mixed IPv4/IPv6 address family")
	}
	var (
		frame       []byte
		ipHeaderLen int
	)
	ipPayloadLen := tcpHeaderLen + len(packetInfo.options) + len(payload)
	if src.Addr().Is4() {
		ipHeaderLen = header.IPv4MinimumSize
		frame = make([]byte, ipHeaderLen+ipPayloadLen)
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
		frame = make([]byte, ipHeaderLen+ipPayloadLen)
		ip := header.IPv6(frame[:ipHeaderLen])
		ip.Encode(&header.IPv6Fields{
			PayloadLength:     uint16(ipPayloadLen),
			TransportProtocol: header.TCPProtocolNumber,
			HopLimit:          defaultTTL,
			SrcAddr:           src.Addr(),
			DstAddr:           dst.Addr(),
		})
	}
	encodeTCP(frame, ipHeaderLen, src, dst, packetInfo, payload)
	return frame
}

func encodeTCP(frame []byte, ipHeaderLen int, src, dst netip.AddrPort, packetInfo spoofPacketInfo, payload []byte) {
	tcp := header.TCP(frame[ipHeaderLen:])
	copy(frame[ipHeaderLen+tcpHeaderLen:], packetInfo.options)
	optionsLen := len(packetInfo.options)
	copy(frame[ipHeaderLen+tcpHeaderLen+optionsLen:], payload)
	tcp.Encode(&header.TCPFields{
		SrcPort:    src.Port(),
		DstPort:    dst.Port(),
		SeqNum:     packetInfo.seqNum,
		AckNum:     packetInfo.ackNum,
		DataOffset: uint8(tcpHeaderLen + optionsLen),
		Flags:      header.TCPFlagAck | header.TCPFlagPsh,
		WindowSize: defaultWindowSize,
	})
	applyTCPChecksum(tcp, src.Addr(), dst.Addr(), payload, packetInfo.corrupt)
}

func buildSpoofFrame(method Method, src, dst netip.AddrPort, sendNext, receiveNext, timestamp uint32, payload []byte) ([]byte, error) {
	packetinfo, err := resolveSpoofPacketInfo(method, sendNext, receiveNext, timestamp, payload)
	if err != nil {
		return nil, err
	}
	return buildTCPSegment(src, dst, packetinfo, payload), nil
}

// buildSpoofTCPSegment returns a TCP segment without an IP header, for
// platforms where the kernel synthesises the IP header (darwin IPv6).
func buildSpoofTCPSegment(method Method, src, dst netip.AddrPort, sendNext, receiveNext, timestamp uint32, payload []byte) ([]byte, error) {
	packetinfo, err := resolveSpoofPacketInfo(method, sendNext, receiveNext, timestamp, payload)
	if err != nil {
		return nil, err
	}
	segment := make([]byte, tcpHeaderLen+len(payload))
	encodeTCP(segment, 0, src, dst, packetinfo, payload)
	return segment, nil
}

func resolveSpoofPacketInfo(method Method, sendNext, receiveNext, timestamp uint32, payload []byte) (spoofPacketInfo, error) {
	var packetinfo spoofPacketInfo

	switch method {
	case MethodWrongSequence:
		packetinfo.seqNum = sendNext - uint32(len(payload))
		packetinfo.ackNum = receiveNext
		packetinfo.corrupt = false
	case MethodWrongChecksum:
		packetinfo.seqNum = sendNext
		packetinfo.ackNum = receiveNext
		packetinfo.corrupt = true
	case MethodWrongAcknowledgment:
		packetinfo.seqNum = sendNext
		packetinfo.ackNum = receiveNext - uint32(defaultWindowSize/2)
		packetinfo.corrupt = false
	case MethodWrongMD5Sig:
		packetinfo.seqNum = sendNext
		packetinfo.ackNum = receiveNext
		packetinfo.corrupt = false
		packetinfo.options = []byte{19, 18, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	case MethodWrongTimestamp:
		packetinfo.seqNum = sendNext
		packetinfo.ackNum = receiveNext
		packetinfo.corrupt = false
		packetinfo.options = []byte{8, 10, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
		binary.BigEndian.PutUint32(packetinfo.options[2:], timestamp-3600000)
	default:
		return packetinfo, E.New("tls_spoof: unknown method ", method)
	}

	return packetinfo, nil
}

func applyTCPChecksum(tcp header.TCP, srcAddr, dstAddr netip.Addr, payload []byte, corrupt bool) {
	tcpLen := int(tcp.DataOffset()) + len(payload)
	pseudo := header.PseudoHeaderChecksum(header.TCPProtocolNumber, srcAddr.AsSlice(), dstAddr.AsSlice(), uint16(tcpLen))
	payloadChecksum := checksum.Checksum(payload, 0)
	tcpChecksum := ^tcp.CalculateChecksum(checksum.Combine(pseudo, payloadChecksum))
	if corrupt {
		tcpChecksum ^= 0xFFFF
	}
	tcp.SetChecksum(tcpChecksum)
}
