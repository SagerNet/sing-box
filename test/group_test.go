package main

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/protocol/group"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks"

	"github.com/stretchr/testify/require"
)

func startSelectorInstance(t *testing.T, interruptExistConnections bool) *group.Selector {
	instance := startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeSOCKS,
				Tag:  "socks-in",
				Options: &option.SocksInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: clientPort,
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeSelector,
				Tag:  "sel",
				Options: &option.SelectorOutboundOptions{
					Outbounds:                 []string{"memA", "memB"},
					Default:                   "memA",
					InterruptExistConnections: interruptExistConnections,
				},
			},
			{
				Type: C.TypeDirect,
				Tag:  "memA",
			},
			{
				Type: C.TypeDirect,
				Tag:  "memB",
			},
		},
		Route: &option.RouteOptions{
			Final: "sel",
		},
	})
	outbound, loaded := instance.Outbound().Outbound("sel")
	require.True(t, loaded)
	selector, isSelector := outbound.(*group.Selector)
	require.True(t, isSelector)
	return selector
}

func startTCPEchoServer(t *testing.T, port uint16) {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: localIP.AsSlice(), Port: int(port)})
	require.NoError(t, err)
	t.Cleanup(func() {
		listener.Close()
	})
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				buffer := make([]byte, 1024)
				for {
					n, err := conn.Read(buffer)
					if err != nil {
						conn.Close()
						return
					}
					_, err = conn.Write(buffer[:n])
					if err != nil {
						conn.Close()
						return
					}
				}
			}()
		}
	}()
}

func startUDPEchoServer(t *testing.T, port uint16) chan net.Addr {
	packetConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: localIP.AsSlice(), Port: int(port)})
	require.NoError(t, err)
	t.Cleanup(func() {
		packetConn.Close()
	})
	sourceCh := make(chan net.Addr, 16)
	go func() {
		buffer := make([]byte, 1024)
		for {
			n, addr, err := packetConn.ReadFrom(buffer)
			if err != nil {
				close(sourceCh)
				return
			}
			sourceCh <- addr
			_, err = packetConn.WriteTo(buffer[:n], addr)
			if err != nil {
				return
			}
		}
	}()
	return sourceCh
}

func tcpPingPong(t *testing.T, conn net.Conn) {
	_, err := conn.Write([]byte("ping"))
	require.NoError(t, err)
	buffer := make([]byte, 4)
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, err = conn.Read(buffer)
	require.NoError(t, err)
	require.Equal(t, []byte("ping"), buffer)
	require.NoError(t, conn.SetReadDeadline(time.Time{}))
}

func TestSelectorInterruptRoutedConnection(t *testing.T) {
	selector := startSelectorInstance(t, true)
	startTCPEchoServer(t, testPort)

	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", clientPort), socks.Version5, "", "")
	conn, err := dialer.DialContext(context.Background(), "tcp", M.ParseSocksaddrHostPort("127.0.0.1", testPort))
	require.NoError(t, err)
	defer conn.Close()
	tcpPingPong(t, conn)

	require.True(t, selector.SelectOutbound("memB"))

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)))
	_, err = conn.Read(make([]byte, 4))
	require.Error(t, err, "existing connection routed through selector must be interrupted")
	var netErr net.Error
	if errors.As(err, &netErr) {
		require.False(t, netErr.Timeout(), "existing connection routed through selector must be interrupted")
	}
}

func TestSelectorInterruptRoutedPacketConnection(t *testing.T) {
	selector := startSelectorInstance(t, true)
	sourceCh := startUDPEchoServer(t, testPort)

	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", clientPort), socks.Version5, "", "")
	packetConn, err := dialer.ListenPacket(context.Background(), M.ParseSocksaddrHostPort("127.0.0.1", testPort))
	require.NoError(t, err)
	defer packetConn.Close()

	serverAddr := &net.UDPAddr{IP: localIP.AsSlice(), Port: int(testPort)}
	buffer := make([]byte, 1024)

	_, err = packetConn.WriteTo([]byte("ping"), serverAddr)
	require.NoError(t, err)
	require.NoError(t, packetConn.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, _, err = packetConn.ReadFrom(buffer)
	require.NoError(t, err)
	firstSource := <-sourceCh

	require.True(t, selector.SelectOutbound("memB"))
	// The interrupted NAT session cannot signal the SOCKS client; instead verify
	// that a subsequent packet no longer flows over the old outbound socket.
	_, err = packetConn.WriteTo([]byte("ping"), serverAddr)
	require.NoError(t, err)
	require.NoError(t, packetConn.SetReadDeadline(time.Now().Add(3*time.Second)))
	_, _, err = packetConn.ReadFrom(buffer)
	if err == nil {
		secondSource := <-sourceCh
		require.NotEqual(t, firstSource.String(), secondSource.String(),
			"packet connection routed through selector must be interrupted")
	}
}

func TestSelectorKeepRoutedConnection(t *testing.T) {
	selector := startSelectorInstance(t, false)
	startTCPEchoServer(t, testPort)

	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", clientPort), socks.Version5, "", "")
	conn, err := dialer.DialContext(context.Background(), "tcp", M.ParseSocksaddrHostPort("127.0.0.1", testPort))
	require.NoError(t, err)
	defer conn.Close()
	tcpPingPong(t, conn)

	require.True(t, selector.SelectOutbound("memB"))

	tcpPingPong(t, conn)
}
