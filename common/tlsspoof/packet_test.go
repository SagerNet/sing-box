package tlsspoof

import (
	"net/netip"
	"testing"

	"github.com/sagernet/sing-tun/gtcpip"
	"github.com/sagernet/sing-tun/gtcpip/checksum"
	"github.com/sagernet/sing-tun/gtcpip/header"

	"github.com/stretchr/testify/require"
)

func TestBuildTCPSegment_IPv4_ValidChecksum(t *testing.T) {
	t.Parallel()
	src := netip.MustParseAddrPort("10.0.0.1:54321")
	dst := netip.MustParseAddrPort("1.2.3.4:443")
	payload := []byte("fake-client-hello")
	frame := buildTCPSegment(src, dst, 100_000, 200_000, payload, false, true)

	ip := header.IPv4(frame[:header.IPv4MinimumSize])
	require.True(t, ip.IsChecksumValid())

	tcp := header.TCP(frame[header.IPv4MinimumSize:])
	payloadChecksum := checksum.Checksum(payload, 0)
	require.True(t, tcp.IsChecksumValid(
		tcpip.AddrFrom4(src.Addr().As4()),
		tcpip.AddrFrom4(dst.Addr().As4()),
		payloadChecksum,
		uint16(len(payload)),
	))
}

func TestBuildTCPSegment_IPv4_CorruptChecksum(t *testing.T) {
	t.Parallel()
	src := netip.MustParseAddrPort("10.0.0.1:54321")
	dst := netip.MustParseAddrPort("1.2.3.4:443")
	payload := []byte("fake-client-hello")
	frame := buildTCPSegment(src, dst, 100_000, 200_000, payload, true, true)

	tcp := header.TCP(frame[header.IPv4MinimumSize:])
	payloadChecksum := checksum.Checksum(payload, 0)
	require.False(t, tcp.IsChecksumValid(
		tcpip.AddrFrom4(src.Addr().As4()),
		tcpip.AddrFrom4(dst.Addr().As4()),
		payloadChecksum,
		uint16(len(payload)),
	))
	// IP checksum must still be valid so the router forwards the packet.
	require.True(t, header.IPv4(frame[:header.IPv4MinimumSize]).IsChecksumValid())
}

func TestBuildTCPSegment_IPv6_ValidChecksum(t *testing.T) {
	t.Parallel()
	src := netip.MustParseAddrPort("[fe80::1]:54321")
	dst := netip.MustParseAddrPort("[2606:4700::1]:443")
	payload := []byte("fake-client-hello")
	frame := buildTCPSegment(src, dst, 0xDEADBEEF, 0x12345678, payload, false, true)

	tcp := header.TCP(frame[header.IPv6MinimumSize:])
	payloadChecksum := checksum.Checksum(payload, 0)
	require.True(t, tcp.IsChecksumValid(
		tcpip.AddrFrom16(src.Addr().As16()),
		tcpip.AddrFrom16(dst.Addr().As16()),
		payloadChecksum,
		uint16(len(payload)),
	))
}

func TestBuildTCPSegment_MixedFamilyPanics(t *testing.T) {
	t.Parallel()
	src := netip.MustParseAddrPort("10.0.0.1:54321")
	dst := netip.MustParseAddrPort("[2606:4700::1]:443")
	require.Panics(t, func() {
		buildTCPSegment(src, dst, 0, 0, nil, false, true)
	})
}

func TestBuildSpoofFrames_NoSplitWhenPayloadFitsMSS(t *testing.T) {
	t.Parallel()
	src := netip.MustParseAddrPort("10.0.0.1:54321")
	dst := netip.MustParseAddrPort("1.2.3.4:443")
	payload := make([]byte, 400)
	frames, err := buildSpoofFrames(MethodWrongSequence, src, dst, 10_000, 20_000, payload, 1360)
	require.NoError(t, err)
	require.Len(t, frames, 1)
	require.Equal(t, header.IPv4MinimumSize+header.TCPMinimumSize+len(payload), len(frames[0]))
}

func TestBuildSpoofFrames_SplitsOversizedPayload(t *testing.T) {
	t.Parallel()
	src := netip.MustParseAddrPort("10.0.0.1:54321")
	dst := netip.MustParseAddrPort("1.2.3.4:443")
	payload := make([]byte, 1508)
	for i := range payload {
		payload[i] = byte(i)
	}
	const mss = 1360
	const sendNext uint32 = 1
	frames, err := buildSpoofFrames(MethodWrongSequence, src, dst, sendNext, 777, payload, mss)
	require.NoError(t, err)
	require.Len(t, frames, 2)

	// Every frame's IP payload must fit an unfragmented 1500-byte MTU.
	for _, frame := range frames {
		require.LessOrEqual(t, len(frame), header.IPv4MinimumSize+header.TCPMinimumSize+mss)
	}

	first := header.TCP(frames[0][header.IPv4MinimumSize:])
	second := header.TCP(frames[1][header.IPv4MinimumSize:])
	firstPayloadLen := len(frames[0]) - header.IPv4MinimumSize - header.TCPMinimumSize
	secondPayloadLen := len(frames[1]) - header.IPv4MinimumSize - header.TCPMinimumSize
	require.Equal(t, mss, firstPayloadLen)
	require.Equal(t, len(payload)-mss, secondPayloadLen)

	// Wrong-sequence: seqs are offsets from sendNext-len(payload).
	baseSeq := sendNext - uint32(len(payload))
	require.Equal(t, baseSeq, first.SequenceNumber())
	require.Equal(t, baseSeq+uint32(mss), second.SequenceNumber())

	// ACK on all, PSH only on the last chunk.
	require.True(t, first.Flags().Contains(header.TCPFlagAck))
	require.False(t, first.Flags().Contains(header.TCPFlagPsh))
	require.True(t, second.Flags().Contains(header.TCPFlagAck))
	require.True(t, second.Flags().Contains(header.TCPFlagPsh))

	// Checksums valid for the wrong-sequence method (only seq is wrong).
	firstChunkChecksum := checksum.Checksum(payload[:mss], 0)
	require.True(t, first.IsChecksumValid(
		tcpip.AddrFrom4(src.Addr().As4()),
		tcpip.AddrFrom4(dst.Addr().As4()),
		firstChunkChecksum,
		uint16(firstPayloadLen),
	))
	secondChunkChecksum := checksum.Checksum(payload[mss:], 0)
	require.True(t, second.IsChecksumValid(
		tcpip.AddrFrom4(src.Addr().As4()),
		tcpip.AddrFrom4(dst.Addr().As4()),
		secondChunkChecksum,
		uint16(secondPayloadLen),
	))

	// Concatenating the payloads reproduces the original.
	reassembled := make([]byte, 0, len(payload))
	reassembled = append(reassembled, frames[0][header.IPv4MinimumSize+header.TCPMinimumSize:]...)
	reassembled = append(reassembled, frames[1][header.IPv4MinimumSize+header.TCPMinimumSize:]...)
	require.Equal(t, payload, reassembled)
}

func TestBuildSpoofFrames_WrongChecksumCorruptsEveryChunk(t *testing.T) {
	t.Parallel()
	src := netip.MustParseAddrPort("10.0.0.1:54321")
	dst := netip.MustParseAddrPort("1.2.3.4:443")
	payload := make([]byte, 1508)
	const mss = 1360
	const sendNext uint32 = 5000
	frames, err := buildSpoofFrames(MethodWrongChecksum, src, dst, sendNext, 777, payload, mss)
	require.NoError(t, err)
	require.Len(t, frames, 2)

	// Wrong-checksum: base seq equals sendNext, subsequent chunks offset by mss.
	first := header.TCP(frames[0][header.IPv4MinimumSize:])
	second := header.TCP(frames[1][header.IPv4MinimumSize:])
	require.Equal(t, sendNext, first.SequenceNumber())
	require.Equal(t, sendNext+uint32(mss), second.SequenceNumber())

	for idx, frame := range frames {
		tcp := header.TCP(frame[header.IPv4MinimumSize:])
		chunkPayloadLen := len(frame) - header.IPv4MinimumSize - header.TCPMinimumSize
		chunkChecksum := checksum.Checksum(frame[header.IPv4MinimumSize+header.TCPMinimumSize:], 0)
		require.False(t, tcp.IsChecksumValid(
			tcpip.AddrFrom4(src.Addr().As4()),
			tcpip.AddrFrom4(dst.Addr().As4()),
			chunkChecksum,
			uint16(chunkPayloadLen),
		), "chunk %d must have corrupt TCP checksum", idx)
		// IP checksum stays valid so routers forward the packet.
		require.True(t, header.IPv4(frame[:header.IPv4MinimumSize]).IsChecksumValid(), "chunk %d IPv4 checksum", idx)
	}
}

func TestBuildSpoofFrames_EmptyPayloadReturnsNoFrames(t *testing.T) {
	t.Parallel()
	src := netip.MustParseAddrPort("10.0.0.1:54321")
	dst := netip.MustParseAddrPort("1.2.3.4:443")
	frames, err := buildSpoofFrames(MethodWrongSequence, src, dst, 1, 1, nil, 1360)
	require.NoError(t, err)
	require.Empty(t, frames)
}

func TestBuildSpoofFrames_RejectsNonPositiveMSS(t *testing.T) {
	t.Parallel()
	src := netip.MustParseAddrPort("10.0.0.1:54321")
	dst := netip.MustParseAddrPort("1.2.3.4:443")
	_, err := buildSpoofFrames(MethodWrongSequence, src, dst, 1, 1, []byte("x"), 0)
	require.Error(t, err)
	_, err = buildSpoofFrames(MethodWrongSequence, src, dst, 1, 1, []byte("x"), -1)
	require.Error(t, err)
}

func TestBuildSpoofSegments_IPv6NoIPHeaderSplits(t *testing.T) {
	t.Parallel()
	src := netip.MustParseAddrPort("[fe80::1]:54321")
	dst := netip.MustParseAddrPort("[2606:4700::1]:443")
	payload := make([]byte, 1500)
	const mss = 1200
	segments, err := buildSpoofSegments(MethodWrongSequence, src, dst, 100, 200, payload, mss)
	require.NoError(t, err)
	require.Len(t, segments, 2)
	// Segments have no IP header.
	require.Equal(t, header.TCPMinimumSize+mss, len(segments[0]))
	require.Equal(t, header.TCPMinimumSize+(len(payload)-mss), len(segments[1]))

	first := header.TCP(segments[0])
	second := header.TCP(segments[1])
	require.False(t, first.Flags().Contains(header.TCPFlagPsh))
	require.True(t, second.Flags().Contains(header.TCPFlagPsh))
}
