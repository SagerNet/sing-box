//go:build windows && (amd64 || 386)

package tlsspoof

import (
	"net/netip"
	"testing"

	"github.com/sagernet/sing-tun/gtcpip/header"

	"github.com/stretchr/testify/require"
)

func TestParseTCPFieldsIPv4Valid(t *testing.T) {
	t.Parallel()
	src := netip.MustParseAddrPort("10.0.0.1:54321")
	dst := netip.MustParseAddrPort("1.2.3.4:443")
	payload := []byte("hello")
	frame := buildTCPSegment(src, dst, 1000, 2000, payload, false)

	seq, ack, payloadLen, ok := parseTCPFields(frame, false)
	require.True(t, ok)
	require.Equal(t, uint32(1000), seq)
	require.Equal(t, uint32(2000), ack)
	require.Equal(t, len(payload), payloadLen)
}

func TestParseTCPFieldsIPv4NoPayload(t *testing.T) {
	t.Parallel()
	src := netip.MustParseAddrPort("10.0.0.1:54321")
	dst := netip.MustParseAddrPort("1.2.3.4:443")
	frame := buildTCPSegment(src, dst, 42, 100, nil, false)

	seq, ack, payloadLen, ok := parseTCPFields(frame, false)
	require.True(t, ok)
	require.Equal(t, uint32(42), seq)
	require.Equal(t, uint32(100), ack)
	require.Equal(t, 0, payloadLen)
}

func TestParseTCPFieldsIPv6Valid(t *testing.T) {
	t.Parallel()
	src := netip.MustParseAddrPort("[fe80::1]:54321")
	dst := netip.MustParseAddrPort("[2606:4700::1]:443")
	payload := []byte("hello-v6")
	frame := buildTCPSegment(src, dst, 0xDEADBEEF, 0x12345678, payload, false)

	seq, ack, payloadLen, ok := parseTCPFields(frame, true)
	require.True(t, ok)
	require.Equal(t, uint32(0xDEADBEEF), seq)
	require.Equal(t, uint32(0x12345678), ack)
	require.Equal(t, len(payload), payloadLen)
}

func TestParseTCPFieldsIPv4TooShort(t *testing.T) {
	t.Parallel()
	_, _, _, ok := parseTCPFields(make([]byte, header.IPv4MinimumSize+header.TCPMinimumSize-1), false)
	require.False(t, ok)
}

func TestParseTCPFieldsIPv6TooShort(t *testing.T) {
	t.Parallel()
	_, _, _, ok := parseTCPFields(make([]byte, header.IPv6MinimumSize+header.TCPMinimumSize-1), true)
	require.False(t, ok)
}

// buildTCPSegment only produces TCP; a UDP packet hitting parseTCPFields
// (for example from a mis-specified filter) must be rejected.
func TestParseTCPFieldsIPv4WrongProtocol(t *testing.T) {
	t.Parallel()
	frame := make([]byte, header.IPv4MinimumSize+header.TCPMinimumSize)
	ip := header.IPv4(frame[:header.IPv4MinimumSize])
	ip.Encode(&header.IPv4Fields{
		TotalLength: uint16(len(frame)),
		TTL:         64,
		Protocol:    17, // UDP
		SrcAddr:     netip.MustParseAddr("10.0.0.1"),
		DstAddr:     netip.MustParseAddr("10.0.0.2"),
	})
	_, _, _, ok := parseTCPFields(frame, false)
	require.False(t, ok)
}

func TestParseTCPFieldsIPv6WrongProtocol(t *testing.T) {
	t.Parallel()
	frame := make([]byte, header.IPv6MinimumSize+header.TCPMinimumSize)
	ip := header.IPv6(frame[:header.IPv6MinimumSize])
	ip.Encode(&header.IPv6Fields{
		PayloadLength:     header.TCPMinimumSize,
		TransportProtocol: 17, // UDP
		HopLimit:          64,
		SrcAddr:           netip.MustParseAddr("fe80::1"),
		DstAddr:           netip.MustParseAddr("fe80::2"),
	})
	_, _, _, ok := parseTCPFields(frame, true)
	require.False(t, ok)
}

// ihl > 20 must not read past the TCP slice. Build an IPv4 packet with
// options header but truncate so ihl*4 + TCPMinimumSize exceeds len.
func TestParseTCPFieldsIPv4OptionsOverflow(t *testing.T) {
	t.Parallel()
	// Start with a valid IPv4+TCP frame, then lie about the header length.
	src := netip.MustParseAddrPort("10.0.0.1:1")
	dst := netip.MustParseAddrPort("10.0.0.2:2")
	frame := buildTCPSegment(src, dst, 0, 0, []byte("x"), false)
	ip := header.IPv4(frame[:header.IPv4MinimumSize])
	// ihl=15 → 60 bytes of IP header claimed, but buffer only has 20.
	ip.SetHeaderLength(60)
	_, _, _, ok := parseTCPFields(frame, false)
	require.False(t, ok)
}
