package main

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/http3"
	"github.com/sagernet/sing-box"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/debug"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

var globalCtx context.Context

func init() {
	globalCtx = include.Context(context.Background())
}

func startInstance(t *testing.T, options option.Options) *box.Box {
	if debug.Enabled {
		options.Log = &option.LogOptions{
			Level: "trace",
		}
	} else {
		options.Log = &option.LogOptions{
			Level: "warning",
		}
	}
	ctx, cancel := context.WithCancel(globalCtx)
	var instance *box.Box
	var err error
	for retry := 0; retry < 3; retry++ {
		instance, err = box.New(box.Options{
			Context: ctx,
			Options: options,
		})
		require.NoError(t, err)
		err = instance.Start()
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		break
	}
	require.NoError(t, err)
	t.Cleanup(func() {
		instance.Close()
		cancel()
	})
	return instance
}

func testSuit(t *testing.T, clientPort uint16, testPort uint16) {
	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", clientPort), socks.Version5, "", "")
	dialTCP := func() (net.Conn, error) {
		return dialer.DialContext(context.Background(), "tcp", M.ParseSocksaddrHostPort("127.0.0.1", testPort))
	}
	dialUDP := func() (net.PacketConn, error) {
		return dialer.ListenPacket(context.Background(), M.ParseSocksaddrHostPort("127.0.0.1", testPort))
	}
	require.NoError(t, testPingPongWithConn(t, testPort, dialTCP))
	require.NoError(t, testPingPongWithPacketConn(t, testPort, dialUDP))
	require.NoError(t, testLargeDataWithConn(t, testPort, dialTCP))
	require.NoError(t, testLargeDataWithPacketConn(t, testPort, dialUDP))

	// require.NoError(t, testPacketConnTimeout(t, dialUDP))
}

func testQUIC(t *testing.T, clientPort uint16) {
	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", clientPort), socks.Version5, "", "")
	client := &http.Client{
		Transport: &http3.RoundTripper{
			Dial: func(ctx context.Context, addr string, tlsCfg *tls.Config, cfg *quic.Config) (*quic.Conn, error) {
				destination := M.ParseSocksaddr(addr)
				udpConn, err := dialer.DialContext(ctx, N.NetworkUDP, destination)
				if err != nil {
					return nil, err
				}
				return quic.DialEarly(ctx, udpConn.(net.PacketConn), destination, tlsCfg, cfg)
			},
		},
	}
	response, err := client.Get("https://cloudflare.com/cdn-cgi/trace")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	content, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	println(string(content))
}

func testSuitLargeUDP(t *testing.T, clientPort uint16, testPort uint16) {
	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", clientPort), socks.Version5, "", "")
	dialTCP := func() (net.Conn, error) {
		return dialer.DialContext(context.Background(), "tcp", M.ParseSocksaddrHostPort("127.0.0.1", testPort))
	}
	dialUDP := func() (net.PacketConn, error) {
		return dialer.ListenPacket(context.Background(), M.ParseSocksaddrHostPort("127.0.0.1", testPort))
	}
	require.NoError(t, testPingPongWithConn(t, testPort, dialTCP))
	require.NoError(t, testPingPongWithPacketConn(t, testPort, dialUDP))
	require.NoError(t, testLargeDataWithConn(t, testPort, dialTCP))
	require.NoError(t, testLargeDataWithPacketConn(t, testPort, dialUDP))
	require.NoError(t, testLargeDataWithPacketConnSize(t, testPort, 4096, dialUDP))
}

func testTCP(t *testing.T, clientPort uint16, testPort uint16) {
	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", clientPort), socks.Version5, "", "")
	dialTCP := func() (net.Conn, error) {
		return dialer.DialContext(context.Background(), "tcp", M.ParseSocksaddrHostPort("127.0.0.1", testPort))
	}
	require.NoError(t, testPingPongWithConn(t, testPort, dialTCP))
	require.NoError(t, testLargeDataWithConn(t, testPort, dialTCP))
}

func testSuitSimple(t *testing.T, clientPort uint16, testPort uint16) {
	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", clientPort), socks.Version5, "", "")
	dialTCP := func() (net.Conn, error) {
		return dialer.DialContext(context.Background(), "tcp", M.ParseSocksaddrHostPort("127.0.0.1", testPort))
	}
	dialUDP := func() (net.PacketConn, error) {
		return dialer.ListenPacket(context.Background(), M.ParseSocksaddrHostPort("127.0.0.1", testPort))
	}
	require.NoError(t, testPingPongWithConn(t, testPort, dialTCP))
	require.NoError(t, testPingPongWithPacketConn(t, testPort, dialUDP))
	require.NoError(t, testPingPongWithConn(t, testPort, dialTCP))
	require.NoError(t, testPingPongWithPacketConn(t, testPort, dialUDP))
}

func testSuitSimple1(t *testing.T, clientPort uint16, testPort uint16) {
	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", clientPort), socks.Version5, "", "")
	dialTCP := func() (net.Conn, error) {
		return dialer.DialContext(context.Background(), "tcp", M.ParseSocksaddrHostPort("127.0.0.1", testPort))
	}
	dialUDP := func() (net.PacketConn, error) {
		return dialer.ListenPacket(context.Background(), M.ParseSocksaddrHostPort("127.0.0.1", testPort))
	}
	require.NoError(t, testPingPongWithConn(t, testPort, dialTCP))
	if !C.IsDarwin {
		require.NoError(t, testPingPongWithPacketConn(t, testPort, dialUDP))
	}
	require.NoError(t, testPingPongWithConn(t, testPort, dialTCP))
	if !C.IsDarwin {
		require.NoError(t, testLargeDataWithPacketConn(t, testPort, dialUDP))
	}
}

func testSuitWg(t *testing.T, clientPort uint16, testPort uint16) {
	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", clientPort), socks.Version5, "", "")
	dialTCP := func() (net.Conn, error) {
		return dialer.DialContext(context.Background(), "tcp", M.ParseSocksaddrHostPort("10.0.0.1", testPort))
	}
	dialUDP := func() (net.PacketConn, error) {
		conn, err := dialer.DialContext(context.Background(), "udp", M.ParseSocksaddrHostPort("10.0.0.1", testPort))
		if err != nil {
			return nil, err
		}
		return bufio.NewUnbindPacketConn(conn), nil
	}
	require.NoError(t, testPingPongWithConn(t, testPort, dialTCP))
	require.NoError(t, testPingPongWithPacketConn(t, testPort, dialUDP))
	require.NoError(t, testLargeDataWithConn(t, testPort, dialTCP))
	require.NoError(t, testLargeDataWithPacketConn(t, testPort, dialUDP))
}
