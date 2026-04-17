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
	frame := buildTCPSegment(src, dst, 100_000, 200_000, payload, false)

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
	frame := buildTCPSegment(src, dst, 100_000, 200_000, payload, true)

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
	frame := buildTCPSegment(src, dst, 0xDEADBEEF, 0x12345678, payload, false)

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
		buildTCPSegment(src, dst, 0, 0, nil, false)
	})
}

func TestBuildSpoofFrame_WrongSequence(t *testing.T) {
	t.Parallel()
	src := netip.MustParseAddrPort("10.0.0.1:54321")
	dst := netip.MustParseAddrPort("1.2.3.4:443")
	payload := []byte("fake-client-hello")
	const sendNext uint32 = 10_000
	frame, err := buildSpoofFrame(MethodWrongSequence, src, dst, sendNext, 20_000, payload)
	require.NoError(t, err)

	tcp := header.TCP(frame[header.IPv4MinimumSize:])
	require.Equal(t, sendNext-uint32(len(payload)), tcp.SequenceNumber(),
		"wrong-sequence places the fake at sendNext-len(payload)")
	require.True(t, tcp.Flags().Contains(header.TCPFlagAck|header.TCPFlagPsh))

	// Checksum must still be valid — only the sequence number is wrong.
	payloadChecksum := checksum.Checksum(payload, 0)
	require.True(t, tcp.IsChecksumValid(
		tcpip.AddrFrom4(src.Addr().As4()),
		tcpip.AddrFrom4(dst.Addr().As4()),
		payloadChecksum,
		uint16(len(payload)),
	))
}

func TestBuildSpoofFrame_WrongChecksum(t *testing.T) {
	t.Parallel()
	src := netip.MustParseAddrPort("10.0.0.1:54321")
	dst := netip.MustParseAddrPort("1.2.3.4:443")
	payload := []byte("fake-client-hello")
	const sendNext uint32 = 5_000
	frame, err := buildSpoofFrame(MethodWrongChecksum, src, dst, sendNext, 20_000, payload)
	require.NoError(t, err)

	tcp := header.TCP(frame[header.IPv4MinimumSize:])
	require.Equal(t, sendNext, tcp.SequenceNumber(),
		"wrong-checksum keeps the real sequence number")

	payloadChecksum := checksum.Checksum(payload, 0)
	require.False(t, tcp.IsChecksumValid(
		tcpip.AddrFrom4(src.Addr().As4()),
		tcpip.AddrFrom4(dst.Addr().As4()),
		payloadChecksum,
		uint16(len(payload)),
	))
	require.True(t, header.IPv4(frame[:header.IPv4MinimumSize]).IsChecksumValid(),
		"IPv4 checksum must remain valid so the router forwards the packet")
}

func TestBuildSpoofTCPSegment_EncodesWithoutIPHeader(t *testing.T) {
	t.Parallel()
	src := netip.MustParseAddrPort("[fe80::1]:54321")
	dst := netip.MustParseAddrPort("[2606:4700::1]:443")
	payload := []byte("fake-client-hello")
	segment, err := buildSpoofTCPSegment(MethodWrongSequence, src, dst, 1000, 2000, payload)
	require.NoError(t, err)
	require.Equal(t, tcpHeaderLen+len(payload), len(segment),
		"segment must be TCP header + payload, no IP header")
}
