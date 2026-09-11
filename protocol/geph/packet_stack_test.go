//go:build with_gvisor

package geph

import (
	"context"
	"net/netip"
	"testing"
	"time"

	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func TestPacketStackForwardsTCPIntoGephTransport(t *testing.T) {
	incoming := make(chan []byte, 4)
	outgoing := make(chan []byte, 4)
	stack, err := newPacketStack(incoming, func(packet []byte) error {
		outgoing <- append([]byte(nil), packet...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stack.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	connCh := make(chan error, 1)
	go func() {
		conn, err := stack.DialContext(ctx, N.NetworkTCP, M.SocksaddrFrom(netip.MustParseAddr("198.18.0.1"), 443))
		if err == nil && conn != nil {
			_ = conn.Close()
		}
		connCh <- err
	}()

	select {
	case packet := <-outgoing:
		if len(packet) < 20 || packet[0]>>4 != 4 {
			t.Fatalf("expected IPv4 TCP packet, got %x", packet)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for packet sent to Geph")
	}
}
