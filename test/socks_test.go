package main

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks"

	"github.com/stretchr/testify/require"
)

// TestSOCKSUDPTimeout tests that a SOCKS5 UDP association is properly closed
// after the configured udp_timeout when there is no activity.
//
// This tests the fix for "socks: Fix missing UDP timeout" where metadata.UDPTimeout
// was not being set from the inbound configuration, causing UDP sessions to hang
// indefinitely instead of being closed after the NAT expiration timeout.
func TestSOCKSUDPTimeout(t *testing.T) {
	// Use a short timeout for testing (2 seconds)
	const testTimeout = 2 * time.Second
	udpTimeout := option.UDPTimeoutCompat(testTimeout)

	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeSOCKS,
				Tag:  "socks-in",
				Options: &option.SocksInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: clientPort,
						UDPTimeout: udpTimeout,
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
		},
	})

	// Test that UDP session is closed after idle timeout
	testUDPSessionIdleTimeout(t, clientPort, testPort, testTimeout)
}

// TestMixedUDPTimeout tests that a Mixed inbound UDP association is properly closed
// after the configured udp_timeout when there is no activity.
func TestMixedUDPTimeout(t *testing.T) {
	// Use a short timeout for testing (2 seconds)
	const testTimeout = 2 * time.Second
	udpTimeout := option.UDPTimeoutCompat(testTimeout)

	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: clientPort,
						UDPTimeout: udpTimeout,
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
		},
	})

	// Test that UDP session is closed after idle timeout
	testUDPSessionIdleTimeout(t, clientPort, testPort, testTimeout)
}

// testUDPSessionIdleTimeout verifies that a UDP session through the SOCKS proxy
// is properly closed after being idle for the configured timeout duration.
//
// Due to SOCKS5 protocol limitations, the client's ReadFrom() blocks on the UDP
// socket and doesn't detect when the server closes the association. So we test
// by sending a packet after the timeout and verifying it fails.
//
// Test flow:
// 1. Start a UDP echo server
// 2. Create UDP association through SOCKS5 proxy
// 3. Send a packet and receive response (establish activity)
// 4. Wait for the timeout duration + buffer
// 5. Try to send another packet - should fail because session was closed
func testUDPSessionIdleTimeout(t *testing.T, proxyPort uint16, echoPort uint16, expectedTimeout time.Duration) {
	// Start a simple UDP echo server
	echoServer, err := listenPacket("udp", ":"+F.ToString(echoPort))
	require.NoError(t, err)
	defer echoServer.Close()

	// Echo server goroutine - responds to any packet
	go func() {
		buf := make([]byte, 1024)
		for {
			n, addr, err := echoServer.ReadFrom(buf)
			if err != nil {
				return
			}
			_, _ = echoServer.WriteTo(buf[:n], addr)
		}
	}()

	// Create SOCKS5 client
	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", proxyPort), socks.Version5, "", "")

	// Create a UDP association through the SOCKS proxy
	pc, err := dialer.ListenPacket(context.Background(), M.ParseSocksaddrHostPort("127.0.0.1", echoPort))
	require.NoError(t, err)
	defer pc.Close()

	rAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: int(echoPort)}

	// Send a test packet to establish the session with activity
	_, err = pc.WriteTo([]byte("hello"), rAddr)
	require.NoError(t, err)

	// Read the echo response to confirm session is working
	buf := make([]byte, 1024)
	pc.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, _, err := pc.ReadFrom(buf)
	require.NoError(t, err, "failed to receive echo response")
	require.Equal(t, "hello", string(buf[:n]))
	t.Log("UDP echo successful, session established")

	// Reset deadline
	pc.SetReadDeadline(time.Time{})

	// Wait for the timeout plus some buffer
	waitTime := expectedTimeout + time.Second
	t.Logf("Waiting %v for UDP session to timeout...", waitTime)
	time.Sleep(waitTime)

	// Now try to send another packet - if the session was properly timed out,
	// this should fail or the echo response should not arrive
	_, err = pc.WriteTo([]byte("after-timeout"), rAddr)
	if err != nil {
		t.Logf("Write after timeout correctly failed: %v", err)
		return
	}

	// If write succeeded, try to read the response
	// It should timeout because the server closed the session
	pc.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, _, err = pc.ReadFrom(buf)

	if err != nil {
		t.Logf("Read after timeout correctly failed: %v", err)
		return
	}

	// If we got a response, the session wasn't closed - this is a failure
	t.Fatalf("UDP session should have timed out after %v, but received response: %s",
		expectedTimeout, string(buf[:n]))
}
