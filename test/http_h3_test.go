//go:build with_quic

package main

import (
	std_bufio "bufio"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/http3"
	sTLS "github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	sHTTP "github.com/sagernet/sing-box/transport/http"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks"

	"github.com/stretchr/testify/require"
)

func dialHTTP3Proxy(t *testing.T, port uint16, enableDatagrams bool) *http3.ClientConn {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	quicConn, err := quic.DialAddrEarly(ctx, "127.0.0.1:"+strconv.Itoa(int(port)), &tls.Config{
		ServerName:         "example.org",
		InsecureSkipVerify: true,
		NextProtos:         []string{http3.NextProtoH3},
	}, &quic.Config{EnableDatagrams: enableDatagrams})
	require.NoError(t, err)
	transport := &http3.Transport{EnableDatagrams: enableDatagrams}
	clientConn := transport.NewClientConn(quicConn)
	t.Cleanup(func() {
		clientConn.CloseWithError(0, "")
		transport.Close()
	})
	return clientConn
}

func TestHTTPInboundHTTP3(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startTLSHTTPInbound(t, certPem, keyPem, []int{1, 2, 3}, nil)
	origin := newForwardOrigin(t)
	clientConn := dialHTTP3Proxy(t, serverPort, true)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := clientConn.OpenRequestStream(ctx)
	require.NoError(t, err)
	err = stream.SendRequestHeader(&http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Host: origin.host()},
		Host:   origin.host(),
		Header: http.Header{"Proxy-Authorization": []string{proxyAuthorization}},
	})
	require.NoError(t, err)
	response, err := stream.ReadResponse()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	_, err = stream.Write([]byte("GET /hello HTTP/1.1\r\nHost: " + origin.host() + "\r\n\r\n"))
	require.NoError(t, err)
	originResponse, err := http.ReadResponse(std_bufio.NewReader(stream), nil)
	require.NoError(t, err)
	body, err := io.ReadAll(originResponse.Body)
	require.NoError(t, err)
	require.Equal(t, "hello", string(body))
	stream.Close()

	forward, err := clientConn.OpenRequestStream(ctx)
	require.NoError(t, err)
	err = forward.SendRequestHeader(&http.Request{
		Method: http.MethodGet,
		URL:    &url.URL{Scheme: "http", Host: origin.host(), Path: "/hello"},
		Host:   origin.host(),
		Header: http.Header{
			"Proxy-Authorization": []string{proxyAuthorization},
			"User-Agent":          nil,
		},
	})
	require.NoError(t, err)
	require.NoError(t, forward.Close())
	forwardResponse, err := forward.ReadResponse()
	require.NoError(t, err)
	body, err = io.ReadAll(forwardResponse.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, forwardResponse.StatusCode)
	require.Equal(t, "hello", string(body))
	forward.Close()

	rejected, err := clientConn.OpenRequestStream(ctx)
	require.NoError(t, err)
	err = rejected.SendRequestHeader(&http.Request{
		Method: http.MethodGet,
		URL:    &url.URL{Scheme: "https", Host: origin.host(), Path: "/hello"},
		Host:   origin.host(),
		Header: http.Header{"Proxy-Authorization": []string{proxyAuthorization}},
	})
	require.NoError(t, err)
	require.NoError(t, rejected.Close())
	rejectedResponse, err := rejected.ReadResponse()
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, rejectedResponse.StatusCode)
}

func openHTTP3ConnectUDP(t *testing.T, ctx context.Context, clientConn *http3.ClientConn, path string) *http3.RequestStream {
	stream, err := clientConn.OpenRequestStream(ctx)
	require.NoError(t, err)
	err = stream.SendRequestHeader(&http.Request{
		Method: http.MethodConnect,
		Proto:  "connect-udp",
		URL: &url.URL{
			Scheme: "https",
			Host:   "example.org",
			Path:   path,
		},
		Host: "example.org",
		Header: http.Header{
			"Capsule-Protocol":    []string{"?1"},
			"Proxy-Authorization": []string{proxyAuthorization},
		},
	})
	require.NoError(t, err)
	response, err := stream.ReadResponse()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	return stream
}

func TestHTTPInboundConnectUDPHTTP3(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startTLSHTTPInbound(t, certPem, keyPem, []int{1, 2, 3}, nil)
	echo := startUDPEcho(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientConn := dialHTTP3Proxy(t, serverPort, true)
	stream := openHTTP3ConnectUDP(t, ctx, clientConn, connectUDPPath(echo))
	for i := 0; i < 3; i++ {
		err := stream.SendDatagram(append([]byte{0}, []byte("ping")...))
		require.NoError(t, err)
		datagram, err := stream.ReceiveDatagram(ctx)
		require.NoError(t, err)
		require.Equal(t, append([]byte{0}, []byte("ping")...), datagram)
	}
	writeDatagramCapsule(t, stream, []byte("capsule"))
	datagram, err := stream.ReceiveDatagram(ctx)
	require.NoError(t, err)
	require.Equal(t, append([]byte{0}, []byte("capsule")...), datagram)
	err = stream.SendDatagram(append([]byte{0x40, 0x00}, []byte("varint")...))
	require.NoError(t, err)
	datagram, err = stream.ReceiveDatagram(ctx)
	require.NoError(t, err)
	require.Equal(t, append([]byte{0}, []byte("varint")...), datagram)
	stream.Close()

	capsuleConn := dialHTTP3Proxy(t, serverPort, false)
	capsuleStream := openHTTP3ConnectUDP(t, ctx, capsuleConn, connectUDPPath(echo))
	reader := std_bufio.NewReader(capsuleStream)
	for i := 0; i < 3; i++ {
		writeDatagramCapsule(t, capsuleStream, []byte("capsule"))
		require.Equal(t, "capsule", string(readDatagramCapsule(t, reader)))
	}
	capsuleStream.Close()
}

func startHTTP3OnlyInbound(t *testing.T, certPem string, keyPem string) {
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeHTTP,
				Options: &option.HTTPInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					Version: []int{3},
					Users:   []auth.User{{Username: "sekai", Password: "password"}},
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
						TLS: &option.InboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
							KeyPath:         keyPem,
						},
					},
				},
			},
		},
		Outbounds: []option.Outbound{{Type: C.TypeDirect}},
	})
}

func startHTTP3Outbound(t *testing.T, certPem string, disableFallback bool) {
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
					Username:               "sekai",
					Password:               "password",
					Version:                3,
					DisableVersionFallback: disableFallback,
					OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
						TLS: &option.OutboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
						},
					},
				},
			},
		},
	})
}

func TestHTTPOutboundHTTP3(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startHTTP3OnlyInbound(t, certPem, keyPem)
	_, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(int(serverPort)), time.Second)
	require.Error(t, err)
	startHTTP3Outbound(t, certPem, true)
	origin := newForwardOrigin(t)
	client := proxyClient(t, clientPort)
	for i := 0; i < 3; i++ {
		request, err := http.NewRequest(http.MethodGet, origin.url("/hello"), nil)
		require.NoError(t, err)
		request.Header.Set("User-Agent", "")
		response, err := client.Do(request)
		require.NoError(t, err)
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		require.NoError(t, err)
		require.Equal(t, "hello", string(body))
	}
	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", clientPort), socks.Version5, "", "")
	dialUDP := func() (net.PacketConn, error) {
		return dialer.ListenPacket(context.Background(), M.ParseSocksaddrHostPort("127.0.0.1", testPort))
	}
	require.NoError(t, testPingPongWithPacketConn(t, testPort, dialUDP))
	require.NoError(t, testLargeDataWithPacketConn(t, testPort, dialUDP))
}

func TestHTTPOutboundHTTP3Fallback(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeHTTP,
				Options: &option.HTTPInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					Version: []int{1, 2},
					Users:   []auth.User{{Username: "sekai", Password: "password"}},
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
						TLS: &option.InboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
							KeyPath:         keyPem,
						},
					},
				},
			},
		},
		Outbounds: []option.Outbound{{Type: C.TypeDirect}},
	})
	startHTTP3Outbound(t, certPem, false)
	origin := newForwardOrigin(t)
	client := proxyClient(t, clientPort)
	request, err := http.NewRequest(http.MethodGet, origin.url("/hello"), nil)
	require.NoError(t, err)
	request.Header.Set("User-Agent", "")
	response, err := client.Do(request)
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	require.NoError(t, err)
	require.Equal(t, "hello", string(body))
}

func TestHTTPOutboundHTTP3NoFallback(t *testing.T) {
	_, certPem, _ := createSelfSignedCertificate(t, "example.org")
	startHTTP3Outbound(t, certPem, true)
	origin := newForwardOrigin(t)
	client := proxyClient(t, clientPort)
	request, err := http.NewRequest(http.MethodGet, origin.url("/hello"), nil)
	require.NoError(t, err)
	response, err := client.Do(request)
	require.NoError(t, err)
	response.Body.Close()
	require.Equal(t, http.StatusBadGateway, response.StatusCode)
}

func TestHTTPOutboundHTTP3Cancel(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	certificate, err := tls.LoadX509KeyPair(certPem, keyPem)
	require.NoError(t, err)
	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	quicListener, err := quic.ListenEarly(udpConn, &tls.Config{
		Certificates: []tls.Certificate{certificate},
		NextProtos:   []string{http3.NextProtoH3},
	}, &quic.Config{EnableDatagrams: true})
	require.NoError(t, err)
	server := &http3.Server{
		EnableDatagrams: true,
		Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			<-request.Context().Done()
		}),
	}
	go server.ServeListener(quicListener)
	t.Cleanup(func() {
		server.Close()
		udpConn.Close()
	})
	serverAddress := M.SocksaddrFromNet(udpConn.LocalAddr())
	tlsConfig, err := sTLS.NewClient(context.Background(), log.NewNOPFactory().Logger(), serverAddress.AddrString(), option.OutboundTLSOptions{
		Enabled:         true,
		ServerName:      "example.org",
		CertificatePath: certPem,
		ALPN:            []string{http3.NextProtoH3},
	})
	require.NoError(t, err)
	client, err := sHTTP.NewClient(sHTTP.ClientOptions{
		RawDialer:              N.SystemDialer,
		TLSConfig:              tlsConfig,
		Server:                 serverAddress,
		Version:                3,
		DisableVersionFallback: true,
	})
	require.NoError(t, err)
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	startTime := time.Now()
	_, err = client.DialContext(ctx, N.NetworkTCP, M.ParseSocksaddrHostPort("127.0.0.1", testPort))
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(startTime), 5*time.Second)
	packetCtx, packetCancel := context.WithTimeout(context.Background(), time.Second)
	defer packetCancel()
	_, err = client.ListenPacket(packetCtx, M.ParseSocksaddrHostPort("127.0.0.1", testPort))
	require.ErrorIs(t, err, context.DeadlineExceeded)
}
