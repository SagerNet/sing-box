package main

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/control"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks"

	"github.com/stretchr/testify/require"
)

func mkPort(t *testing.T) uint16 {
	var lc net.ListenConfig
	lc.Control = control.ReuseAddr()
	for {
		tcpListener, err := lc.Listen(context.Background(), "tcp", ":0")
		require.NoError(t, err)
		listenPort := M.SocksaddrFromNet(tcpListener.Addr()).Port
		tcpListener.Close()
		udpListener, err := lc.Listen(context.Background(), "tcp", F.ToString(":", listenPort))
		if err != nil {
			continue
		}
		udpListener.Close()
		return listenPort
	}
}

func startInstance(t *testing.T, options option.Options) {
	instance, err := box.New(context.Background(), options)
	require.NoError(t, err)
	require.NoError(t, instance.Start())
	t.Cleanup(func() {
		instance.Close()
	})
	time.Sleep(time.Second)
}

func testSuit(t *testing.T, clientPort uint16, testPort uint16) {
	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", clientPort), socks.Version5, "", "")
	dialTCP := func() (net.Conn, error) {
		return dialer.DialContext(context.Background(), "tcp", M.ParseSocksaddrHostPort("127.0.0.1", testPort))
	}
	dialUDP := func() (net.PacketConn, error) {
		return dialer.ListenPacket(context.Background(), M.ParseSocksaddrHostPort("127.0.0.1", testPort))
	}
	t.Run("tcp", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, testLargeDataWithConn(t, testPort, dialTCP))
	})
	t.Run("udp", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, testLargeDataWithPacketConn(t, testPort, dialUDP))
	})
	// require.NoError(t, testPingPongWithConn(t, testPort, dialTCP))
	// require.NoError(t, testPingPongWithPacketConn(t, testPort, dialUDP))
	// require.NoError(t, testPacketConnTimeout(t, dialUDP))
}
