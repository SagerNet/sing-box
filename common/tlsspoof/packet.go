//go:build linux || darwin || (windows && (amd64 || 386))

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

	tcpOptionMD5Signature       = 19
	tcpOptionMD5SignatureLength = 18
	tcpTimestampBackdate        = 3600000
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

func buildSpoofFrame(method Method, src, dst netip.AddrPort, sendNext, receiveNext, timestamp uint32, tcpOptions, payload []byte) ([]byte, error) {
	packetInfo, err := resolveSpoofPacketInfo(method, sendNext, receiveNext, timestamp, tcpOptions, payload)
	if err != nil {
		return nil, err
	}
	return buildTCPSegment(src, dst, packetInfo, payload), nil
}

func resolveSpoofPacketInfo(method Method, sendNext, receiveNext, timestamp uint32, tcpOptions, payload []byte) (spoofPacketInfo, error) {
	packetInfo := spoofPacketInfo{seqNum: sendNext, ackNum: receiveNext}
	switch method {
	case MethodWrongSequence:
		packetInfo.seqNum = sendNext - uint32(len(payload))
	case MethodWrongChecksum:
		packetInfo.corrupt = true
	case MethodWrongAcknowledgment:
		packetInfo.ackNum = receiveNext - uint32(defaultWindowSize/2)
	case MethodWrongMD5Sig:
		packetInfo.options = buildMD5SignatureOptions()
	case MethodWrongTimestamp:
		packetInfo.options = buildWrongTimestampOptions(timestamp, tcpOptions)
	default:
		return packetInfo, E.New("tls_spoof: unknown method ", method)
	}
	return packetInfo, nil
}

func buildMD5SignatureOptions() []byte {
	options := make([]byte, tcpOptionMD5SignatureLength+2)
	options[0] = tcpOptionMD5Signature
	options[1] = tcpOptionMD5SignatureLength
	return options
}

func buildWrongTimestampOptions(timestamp uint32, tcpOptions []byte) []byte {
	spoofedTimestamp := timestamp
	if spoofedTimestamp > tcpTimestampBackdate {
		spoofedTimestamp -= tcpTimestampBackdate
	} else {
		spoofedTimestamp = 0
	}
	if rewriteTCPOptionTimestamp(tcpOptions, spoofedTimestamp) {
		return tcpOptions
	}
	options := make([]byte, header.TCPOptionTSLength+2)
	header.EncodeTSOption(spoofedTimestamp, 0, options)
	return options
}

// rewriteTCPOptionTimestamp finds the TS option in tcpOptions and writes
// timestamp into its TSVal field in place. The caller must own tcpOptions
// (parseTCPPacket already returns a private copy on Windows).
func rewriteTCPOptionTimestamp(tcpOptions []byte, timestamp uint32) bool {
	for i := 0; i < len(tcpOptions); {
		switch tcpOptions[i] {
		case header.TCPOptionEOL:
			return false
		case header.TCPOptionNOP:
			i++
			continue
		}
		if i+1 >= len(tcpOptions) {
			return false
		}
		optionLen := int(tcpOptions[i+1])
		if optionLen < 2 || i+optionLen > len(tcpOptions) {
			return false
		}
		if tcpOptions[i] == header.TCPOptionTS && optionLen == header.TCPOptionTSLength {
			binary.BigEndian.PutUint32(tcpOptions[i+2:], timestamp)
			return true
		}
		i += optionLen
	}
	return false
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
