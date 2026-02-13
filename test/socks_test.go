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

func TestSOCKSUDPTimeout(t *testing.T) {
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

	testUDPSessionIdleTimeout(t, clientPort, testPort, testTimeout)
}

func TestMixedUDPTimeout(t *testing.T) {
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

	testUDPSessionIdleTimeout(t, clientPort, testPort, testTimeout)
}

func testUDPSessionIdleTimeout(t *testing.T, proxyPort uint16, echoPort uint16, expectedTimeout time.Duration) {
	echoServer, err := listenPacket("udp", ":"+F.ToString(echoPort))
	require.NoError(t, err)
	defer echoServer.Close()

	go func() {
		buffer := make([]byte, 1024)
		for {
			n, address, err := echoServer.ReadFrom(buffer)
			if err != nil {
				return
			}
			_, _ = echoServer.WriteTo(buffer[:n], address)
		}
	}()

	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", proxyPort), socks.Version5, "", "")

	packetConn, err := dialer.ListenPacket(context.Background(), M.ParseSocksaddrHostPort("127.0.0.1", echoPort))
	require.NoError(t, err)
	defer packetConn.Close()

	remoteAddress := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: int(echoPort)}

	_, err = packetConn.WriteTo([]byte("hello"), remoteAddress)
	require.NoError(t, err)

	buffer := make([]byte, 1024)
	packetConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, _, err := packetConn.ReadFrom(buffer)
	require.NoError(t, err, "failed to receive echo response")
	require.Equal(t, "hello", string(buffer[:n]))
	t.Log("UDP echo successful, session established")

	packetConn.SetReadDeadline(time.Time{})

	waitTime := expectedTimeout + time.Second
	t.Logf("Waiting %v for UDP session to timeout...", waitTime)
	time.Sleep(waitTime)

	_, err = packetConn.WriteTo([]byte("after-timeout"), remoteAddress)
	if err != nil {
		t.Logf("Write after timeout correctly failed: %v", err)
		return
	}

	packetConn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, _, err = packetConn.ReadFrom(buffer)

	if err != nil {
		t.Logf("Read after timeout correctly failed: %v", err)
		return
	}

	t.Fatalf("UDP session should have timed out after %v, but received response: %s",
		expectedTimeout, string(buffer[:n]))
}
