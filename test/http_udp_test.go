package main

import (
	std_bufio "bufio"
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"testing"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	sHTTP "github.com/sagernet/sing-box/transport/http"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
)

func appendVarint(b []byte, value uint64) []byte {
	switch {
	case value < 1<<6:
		return append(b, byte(value))
	case value < 1<<14:
		return binary.BigEndian.AppendUint16(b, uint16(value)|0x4000)
	case value < 1<<30:
		return binary.BigEndian.AppendUint32(b, uint32(value)|0x80000000)
	default:
		return binary.BigEndian.AppendUint64(b, value|0xC000000000000000)
	}
}

func readVarint(t *testing.T, reader *std_bufio.Reader) uint64 {
	first, err := reader.ReadByte()
	require.NoError(t, err)
	length := 1 << (first >> 6)
	value := uint64(first & 0x3f)
	for i := 1; i < length; i++ {
		next, err := reader.ReadByte()
		require.NoError(t, err)
		value = value<<8 | uint64(next)
	}
	return value
}

func writeDatagramCapsule(t *testing.T, writer io.Writer, payload []byte) {
	capsule := appendVarint(nil, 0)
	capsule = appendVarint(capsule, uint64(1+len(payload)))
	capsule = append(capsule, 0)
	capsule = append(capsule, payload...)
	_, err := writer.Write(capsule)
	require.NoError(t, err)
}

func readDatagramCapsule(t *testing.T, reader *std_bufio.Reader) []byte {
	capsuleType := readVarint(t, reader)
	require.Equal(t, uint64(0), capsuleType)
	length := readVarint(t, reader)
	contextID, err := reader.ReadByte()
	require.NoError(t, err)
	require.Equal(t, byte(0), contextID)
	payload := make([]byte, length-1)
	_, err = io.ReadFull(reader, payload)
	require.NoError(t, err)
	return payload
}

func startUDPEcho(t *testing.T) *net.UDPAddr {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	t.Cleanup(func() {
		conn.Close()
	})
	go func() {
		buffer := make([]byte, 65535)
		for {
			n, addr, err := conn.ReadFromUDP(buffer)
			if err != nil {
				return
			}
			conn.WriteToUDP(buffer[:n], addr)
		}
	}()
	return conn.LocalAddr().(*net.UDPAddr)
}

func connectUDPPath(target *net.UDPAddr) string {
	return "/.well-known/masque/udp/" + target.IP.String() + "/" + strconv.Itoa(target.Port) + "/"
}

func TestHTTPInboundConnectUDP(t *testing.T) {
	startForwardProxy(t)
	echo := startUDPEcho(t)
	conn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(int(serverPort)))
	require.NoError(t, err)
	defer conn.Close()
	_, err = conn.Write([]byte("GET " + connectUDPPath(echo) + " HTTP/1.1\r\nHost: 127.0.0.1\r\nConnection: Upgrade\r\nUpgrade: connect-udp\r\nCapsule-Protocol: ?1\r\nProxy-Authorization: " + proxyAuthorization + "\r\n\r\n"))
	require.NoError(t, err)
	reader := std_bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)
	require.Equal(t, "connect-udp", response.Header.Get("Upgrade"))
	for i := 0; i < 3; i++ {
		writeDatagramCapsule(t, conn, []byte("ping"))
		require.Equal(t, "ping", string(readDatagramCapsule(t, reader)))
	}
}

func TestHTTPInboundConnectUDPHTTP2(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startTLSHTTPInbound(t, certPem, keyPem, nil, nil)
	echo := startUDPEcho(t)
	clientConn := dialHTTP2Proxy(t, serverPort)
	pipeReader, pipeWriter := io.Pipe()
	request := &http.Request{
		Method: http.MethodConnect,
		URL: &url.URL{
			Scheme: "https",
			Host:   "example.org",
			Path:   connectUDPPath(echo),
		},
		Host: "example.org",
		Header: http.Header{
			":protocol":           []string{"connect-udp"},
			"Capsule-Protocol":    []string{"?1"},
			"Proxy-Authorization": []string{proxyAuthorization},
		},
		Body: pipeReader,
	}
	response, err := clientConn.RoundTrip(request)
	require.NoError(t, err)
	defer response.Body.Close()
	defer pipeWriter.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, "?1", response.Header.Get("Capsule-Protocol"))
	reader := std_bufio.NewReader(response.Body)
	for i := 0; i < 3; i++ {
		writeDatagramCapsule(t, pipeWriter, []byte("ping"))
		require.Equal(t, "ping", string(readDatagramCapsule(t, reader)))
	}
}

func testHTTPOutboundUDP(t *testing.T, tlsOptions *option.OutboundTLSOptions) {
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: clientPort,
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeHTTP,
				Options: &option.HTTPOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					Username: "sekai",
					Password: "password",
					OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
						TLS: tlsOptions,
					},
				},
			},
		},
	})
	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", clientPort), socks.Version5, "", "")
	dialUDP := func() (net.PacketConn, error) {
		return dialer.ListenPacket(context.Background(), M.ParseSocksaddrHostPort("127.0.0.1", testPort))
	}
	require.NoError(t, testPingPongWithPacketConn(t, testPort, dialUDP))
	require.NoError(t, testLargeDataWithPacketConn(t, testPort, dialUDP))
}

func TestHTTPOutboundUDP(t *testing.T) {
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeHTTP,
				Options: &option.HTTPInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					Users: []auth.User{{Username: "sekai", Password: "password"}},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
		},
	})
	testHTTPOutboundUDP(t, nil)
}

func TestHTTPOutboundUDPHTTP2(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startTLSHTTPInbound(t, certPem, keyPem, nil, nil)
	testHTTPOutboundUDP(t, &option.OutboundTLSOptions{
		Enabled:         true,
		ServerName:      "example.org",
		CertificatePath: certPem,
		ALPN:            []string{http2.NextProtoTLS},
	})
}

func TestHTTPOutboundUDPIPv6Path(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		listener.Close()
	})
	pathErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		reader := std_bufio.NewReader(conn)
		request, err := http.ReadRequest(reader)
		if err != nil {
			pathErr <- err
			return
		}
		expectedPath := "/.well-known/masque/udp/::1/" + strconv.Itoa(int(testPort)) + "/"
		expectedRawPath := "/.well-known/masque/udp/%3A%3A1/" + strconv.Itoa(int(testPort)) + "/"
		if request.URL.Path != expectedPath || request.URL.RawPath != expectedRawPath {
			pathErr <- E.New("unexpected request URI: ", request.RequestURI)
			return
		}
		pathErr <- nil
		_, err = conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: connect-udp\r\nCapsule-Protocol: ?1\r\n\r\n"))
		if err != nil {
			return
		}
		io.Copy(conn, reader)
	}()
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: clientPort,
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeHTTP,
				Options: &option.HTTPOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: uint16(listener.Addr().(*net.TCPAddr).Port),
					},
				},
			},
		},
	})
	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", clientPort), socks.Version5, "", "")
	packetConn, err := dialer.ListenPacket(context.Background(), M.ParseSocksaddrHostPort("::1", testPort))
	require.NoError(t, err)
	defer packetConn.Close()
	destination := &net.UDPAddr{IP: net.IPv6loopback, Port: int(testPort)}
	_, err = packetConn.WriteTo([]byte("ping"), destination)
	require.NoError(t, err)
	require.NoError(t, <-pathErr)
	packetConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buffer := make([]byte, 64)
	n, _, err := packetConn.ReadFrom(buffer)
	require.NoError(t, err)
	require.Equal(t, "ping", string(buffer[:n]))
	packetConn.SetReadDeadline(time.Now())
	_, _, err = packetConn.ReadFrom(buffer)
	require.True(t, E.IsTimeout(err))
}

func TestHTTPOutboundUDPDialContext(t *testing.T) {
	startForwardProxy(t)
	echo := startUDPEcho(t)
	client, err := sHTTP.NewClient(sHTTP.ClientOptions{
		Server:   M.ParseSocksaddrHostPort("127.0.0.1", serverPort),
		Username: "sekai",
		Password: "password",
		Version:  1,
	})
	require.NoError(t, err)
	defer client.Close()
	ctx, cancel := context.WithCancel(context.Background())
	packetConn, err := client.ListenPacket(ctx, M.SocksaddrFromNet(echo))
	require.NoError(t, err)
	defer packetConn.Close()
	cancel()
	packetConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, err = packetConn.WriteTo([]byte("ping"), echo)
	require.NoError(t, err)
	buffer := make([]byte, 64)
	n, _, err := packetConn.ReadFrom(buffer)
	require.NoError(t, err)
	require.Equal(t, "ping", string(buffer[:n]))
	packetConn.SetReadDeadline(time.Now())
	_, _, err = packetConn.ReadFrom(buffer)
	require.True(t, E.IsTimeout(err))
}

func TestHTTPOutboundUDPStreamClosed(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
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
				defer conn.Close()
				reader := std_bufio.NewReader(conn)
				_, err := http.ReadRequest(reader)
				if err != nil {
					return
				}
				_, err = conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: connect-udp\r\nCapsule-Protocol: ?1\r\n\r\n"))
				if err != nil {
					return
				}
				io.ReadFull(reader, make([]byte, 7))
			}()
		}
	}()
	client, err := sHTTP.NewClient(sHTTP.ClientOptions{
		Server:  M.SocksaddrFromNet(listener.Addr()),
		Version: 1,
	})
	require.NoError(t, err)
	defer client.Close()
	packetConn, err := client.ListenPacket(context.Background(), M.ParseSocksaddrHostPort("127.0.0.1", testPort))
	require.NoError(t, err)
	defer packetConn.Close()
	_, err = packetConn.WriteTo([]byte("ping"), &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: int(testPort)})
	require.NoError(t, err)
	packetConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, _, err = packetConn.ReadFrom(make([]byte, 64))
	require.Error(t, err)
	require.False(t, E.IsTimeout(err))
}
