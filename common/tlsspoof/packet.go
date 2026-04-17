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

	// Conservative MSS defaults when runtime discovery fails. These assume a
	// 1500-byte path MTU minus the standard IPv4/IPv6 + TCP headers.
	defaultMSSv4 = 1460
	defaultMSSv6 = 1440
)

func defaultMSS(isV4 bool) int {
	if isV4 {
		return defaultMSSv4
	}
	return defaultMSSv6
}

func buildTCPSegment(
	src netip.AddrPort,
	dst netip.AddrPort,
	seqNum uint32,
	ackNum uint32,
	payload []byte,
	corruptChecksum bool,
	setPSH bool,
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
	encodeTCP(frame, ipHeaderLen, src, dst, seqNum, ackNum, payload, corruptChecksum, setPSH)
	return frame
}

func encodeTCP(frame []byte, ipHeaderLen int, src, dst netip.AddrPort, seqNum, ackNum uint32, payload []byte, corruptChecksum bool, setPSH bool) {
	tcp := header.TCP(frame[ipHeaderLen:])
	copy(frame[ipHeaderLen+tcpHeaderLen:], payload)
	flags := header.TCPFlagAck
	if setPSH {
		flags |= header.TCPFlagPsh
	}
	tcp.Encode(&header.TCPFields{
		SrcPort:    src.Port(),
		DstPort:    dst.Port(),
		SeqNum:     seqNum,
		AckNum:     ackNum,
		DataOffset: tcpHeaderLen,
		Flags:      flags,
		WindowSize: defaultWindowSize,
	})
	applyTCPChecksum(tcp, src.Addr(), dst.Addr(), payload, corruptChecksum)
}

// buildSpoofFrames splits payload into <=mss TCP chunks and returns one IP+TCP
// frame per chunk. All chunks carry ACK; only the last carries PSH. Emitting
// <=MSS chunks avoids IP fragmentation on the raw-injection path, which
// otherwise costs enough extra kernel/driver work to lose the wire-order race
// against the kernel's own MSS-segmented real ClientHello.
func buildSpoofFrames(method Method, src, dst netip.AddrPort, sendNext, receiveNext uint32, payload []byte, mss int) ([][]byte, error) {
	if mss <= 0 {
		return nil, E.New("tls_spoof: non-positive mss: ", mss)
	}
	baseSeq, corrupt, err := resolveSpoofSequence(method, sendNext, payload)
	if err != nil {
		return nil, err
	}
	total := len(payload)
	if total == 0 {
		return nil, nil
	}
	frames := make([][]byte, 0, (total+mss-1)/mss)
	for offset := 0; offset < total; offset += mss {
		end := offset + mss
		if end > total {
			end = total
		}
		chunkSeq := baseSeq + uint32(offset)
		isLast := end == total
		frames = append(frames, buildTCPSegment(src, dst, chunkSeq, receiveNext, payload[offset:end], corrupt, isLast))
	}
	return frames, nil
}

// buildSpoofSegments emits TCP-only segments for platforms where the kernel
// synthesises the IP header (darwin IPv6). Splitting semantics match
// buildSpoofFrames.
func buildSpoofSegments(method Method, src, dst netip.AddrPort, sendNext, receiveNext uint32, payload []byte, mss int) ([][]byte, error) {
	if mss <= 0 {
		return nil, E.New("tls_spoof: non-positive mss: ", mss)
	}
	baseSeq, corrupt, err := resolveSpoofSequence(method, sendNext, payload)
	if err != nil {
		return nil, err
	}
	total := len(payload)
	if total == 0 {
		return nil, nil
	}
	segments := make([][]byte, 0, (total+mss-1)/mss)
	for offset := 0; offset < total; offset += mss {
		end := offset + mss
		if end > total {
			end = total
		}
		chunkSeq := baseSeq + uint32(offset)
		isLast := end == total
		segment := make([]byte, tcpHeaderLen+end-offset)
		encodeTCP(segment, 0, src, dst, chunkSeq, receiveNext, payload[offset:end], corrupt, isLast)
		segments = append(segments, segment)
	}
	return segments, nil
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
