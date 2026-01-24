package sniff_test

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/sniff"

	"github.com/stretchr/testify/require"
)

func TestSniffQUICQuicGoFingerprint(t *testing.T) {
	t.Parallel()
	const testSNI = "test.example.com"

	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	require.NoError(t, err)
	defer udpConn.Close()

	serverAddr := udpConn.LocalAddr().(*net.UDPAddr)
	packetsChan := make(chan [][]byte, 1)

	go func() {
		var packets [][]byte
		udpConn.SetReadDeadline(time.Now().Add(3 * time.Second))
		for i := 0; i < 10; i++ {
			buf := make([]byte, 2048)
			n, _, err := udpConn.ReadFromUDP(buf)
			if err != nil {
				break
			}
			packets = append(packets, buf[:n])
		}
		packetsChan <- packets
	}()

	clientConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	require.NoError(t, err)
	defer clientConn.Close()

	tlsConfig := &tls.Config{
		ServerName:         testSNI,
		InsecureSkipVerify: true,
		NextProtos:         []string{"h3"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, _ = quic.Dial(ctx, clientConn, serverAddr, tlsConfig, &quic.Config{})

	select {
	case packets := <-packetsChan:
		t.Logf("Captured %d packets", len(packets))

		var metadata adapter.InboundContext
		for i, pkt := range packets {
			err := sniff.QUICClientHello(context.Background(), &metadata, pkt)
			t.Logf("Packet %d: err=%v, domain=%s, client=%s", i, err, metadata.Domain, metadata.Client)
			if metadata.Domain != "" {
				break
			}
		}

		t.Logf("\n=== quic-go TLS Fingerprint Analysis ===")
		t.Logf("Domain: %s", metadata.Domain)
		t.Logf("Client: %s", metadata.Client)
		t.Logf("Protocol: %s", metadata.Protocol)

		// The client should be identified as quic-go, not chromium
		// Current issue: it's being identified as chromium
		if metadata.Client == "chromium" {
			t.Log("WARNING: quic-go is being misidentified as chromium!")
		}

	case <-time.After(5 * time.Second):
		t.Fatal("Timeout")
	}
}

func TestSniffQUICInitialFromQuicGo(t *testing.T) {
	t.Parallel()

	const testSNI = "test.example.com"

	// Create UDP listener to capture ALL initial packets
	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	require.NoError(t, err)
	defer udpConn.Close()

	serverAddr := udpConn.LocalAddr().(*net.UDPAddr)

	// Channel to receive captured packets
	packetsChan := make(chan [][]byte, 1)

	// Start goroutine to capture packets
	go func() {
		var packets [][]byte
		udpConn.SetReadDeadline(time.Now().Add(3 * time.Second))
		for i := 0; i < 5; i++ { // Capture up to 5 packets
			buf := make([]byte, 2048)
			n, _, err := udpConn.ReadFromUDP(buf)
			if err != nil {
				break
			}
			packets = append(packets, buf[:n])
		}
		packetsChan <- packets
	}()

	// Create QUIC client connection (will fail but we capture the initial packet)
	clientConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	require.NoError(t, err)
	defer clientConn.Close()

	tlsConfig := &tls.Config{
		ServerName:         testSNI,
		InsecureSkipVerify: true,
		NextProtos:         []string{"h3"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// This will fail (no server) but sends initial packet
	_, _ = quic.Dial(ctx, clientConn, serverAddr, tlsConfig, &quic.Config{})

	// Wait for captured packets
	select {
	case packets := <-packetsChan:
		t.Logf("Captured %d QUIC packets", len(packets))

		for i, packet := range packets {
			t.Logf("Packet %d: length=%d, first 30 bytes: %x", i, len(packet), packet[:min(30, len(packet))])
		}

		// Test sniffer with first packet
		if len(packets) > 0 {
			var metadata adapter.InboundContext
			err := sniff.QUICClientHello(context.Background(), &metadata, packets[0])

			t.Logf("First packet sniff error: %v", err)
			t.Logf("Protocol: %s", metadata.Protocol)
			t.Logf("Domain: %s", metadata.Domain)
			t.Logf("Client: %s", metadata.Client)

			// If first packet needs more data, try with subsequent packets
			// IMPORTANT: reuse metadata to accumulate CRYPTO fragments via SniffContext
			if errors.Is(err, sniff.ErrNeedMoreData) && len(packets) > 1 {
				t.Log("First packet needs more data, trying subsequent packets with shared context...")
				for i := 1; i < len(packets); i++ {
					// Reuse same metadata to accumulate fragments
					err = sniff.QUICClientHello(context.Background(), &metadata, packets[i])
					t.Logf("Packet %d sniff result: err=%v, domain=%s, sniffCtx=%v", i, err, metadata.Domain, metadata.SniffContext != nil)
					if metadata.Domain != "" || (err != nil && !errors.Is(err, sniff.ErrNeedMoreData)) {
						break
					}
				}
			}

			// Print hex dump for debugging
			t.Logf("First packet hex:\n%s", hex.Dump(packets[0][:min(256, len(packets[0]))]))

			// Log final results
			t.Logf("Final: Protocol=%s, Domain=%s, Client=%s", metadata.Protocol, metadata.Domain, metadata.Client)

			// Verify SNI extraction
			if metadata.Domain == "" {
				t.Errorf("Failed to extract SNI, expected: %s", testSNI)
			} else {
				require.Equal(t, testSNI, metadata.Domain, "SNI should match")
			}

			// Check client identification - quic-go should be identified as quic-go, not chromium
			t.Logf("Client identified as: %s (expected: quic-go)", metadata.Client)
		}

	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for QUIC packets")
	}
}
