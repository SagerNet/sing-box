package main

import (
	std_bufio "bufio"
	"context"
	"crypto/rand"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	sTLS "github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	sHTTP "github.com/sagernet/sing-box/transport/http"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
)

const proxyAuthorization = "Basic c2VrYWk6cGFzc3dvcmQ="

func startTLSHTTPInbound(t *testing.T, certPem string, keyPem string, versions []int, extraOutbounds []option.Outbound) {
	outbounds := append([]option.Outbound{{Type: C.TypeDirect}}, extraOutbounds...)
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeHTTP,
				Options: &option.HTTPInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					Version: versions,
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
		Outbounds: outbounds,
	})
}

func dialHTTP2Proxy(t *testing.T, port uint16) *http2.ClientConn {
	tlsConn, err := tls.Dial("tcp", "127.0.0.1:"+strconv.Itoa(int(port)), &tls.Config{
		ServerName:         "example.org",
		InsecureSkipVerify: true,
		NextProtos:         []string{http2.NextProtoTLS},
	})
	require.NoError(t, err)
	require.Equal(t, http2.NextProtoTLS, tlsConn.ConnectionState().NegotiatedProtocol)
	clientConn, err := (&http2.Transport{}).NewClientConn(tlsConn)
	require.NoError(t, err)
	t.Cleanup(func() {
		clientConn.Close()
	})
	return clientConn
}

type http2Tunnel struct {
	writer   *io.PipeWriter
	response *http.Response
}

func openHTTP2Tunnel(t *testing.T, clientConn *http2.ClientConn, host string, authorization string) *http2Tunnel {
	pipeReader, pipeWriter := io.Pipe()
	request := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Host: host},
		Host:   host,
		Header: make(http.Header),
		Body:   pipeReader,
	}
	if authorization != "" {
		request.Header.Set("Proxy-Authorization", authorization)
	}
	response, err := clientConn.RoundTrip(request)
	require.NoError(t, err)
	return &http2Tunnel{writer: pipeWriter, response: response}
}

func (t *http2Tunnel) close() {
	t.writer.Close()
	t.response.Body.Close()
}

func TestHTTPInboundHTTP2(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startTLSHTTPInbound(t, certPem, keyPem, nil, nil)
	origin := newForwardOrigin(t)
	clientConn := dialHTTP2Proxy(t, serverPort)

	rejected := openHTTP2Tunnel(t, clientConn, origin.host(), "")
	require.Equal(t, http.StatusProxyAuthRequired, rejected.response.StatusCode)
	require.Contains(t, rejected.response.Header.Get("Proxy-Authenticate"), "Basic")
	rejected.close()

	first := openHTTP2Tunnel(t, clientConn, origin.host(), proxyAuthorization)
	require.Equal(t, http.StatusOK, first.response.StatusCode)
	second := openHTTP2Tunnel(t, clientConn, origin.host(), proxyAuthorization)
	require.Equal(t, http.StatusOK, second.response.StatusCode)
	for _, tunnel := range []*http2Tunnel{second, first} {
		_, err := tunnel.writer.Write([]byte("GET /hello HTTP/1.1\r\nHost: " + origin.host() + "\r\n\r\n"))
		require.NoError(t, err)
		response, err := http.ReadResponse(std_bufio.NewReader(tunnel.response.Body), nil)
		require.NoError(t, err)
		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, "hello", string(body))
		tunnel.close()
	}

	request, err := http.NewRequest(http.MethodGet, origin.url("/hello"), nil)
	require.NoError(t, err)
	request.Header.Set("User-Agent", "")
	request.Header.Set("Proxy-Authorization", proxyAuthorization)
	response, err := clientConn.RoundTrip(request)
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, "hello", string(body))
	require.Equal(t, origin.host(), response.Header.Get("X-Host"))

	response, err = clientConn.RoundTrip(request)
	require.NoError(t, err)
	body, err = io.ReadAll(response.Body)
	response.Body.Close()
	require.NoError(t, err)
	require.Equal(t, "hello", string(body))
	require.Equal(t, int32(4), origin.connections.Load())
}

type http2ProxyServer struct {
	listener    net.Listener
	connections atomic.Int32
	streams     atomic.Int32
}

func startHTTP2ProxyServer(t *testing.T, certPem string, keyPem string) *http2ProxyServer {
	certificate, err := tls.LoadX509KeyPair(certPem, keyPem)
	require.NoError(t, err)
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{certificate},
		NextProtos:   []string{http2.NextProtoTLS},
	})
	require.NoError(t, err)
	server := &http2ProxyServer{listener: listener}
	h2Server := &http2.Server{}
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		server.streams.Add(1)
		if request.Method != http.MethodConnect || request.Header.Get("Proxy-Authorization") != proxyAuthorization {
			writer.WriteHeader(http.StatusProxyAuthRequired)
			return
		}
		conn, err := net.Dial("tcp", request.Host)
		if err != nil {
			writer.WriteHeader(http.StatusBadGateway)
			return
		}
		writer.WriteHeader(http.StatusOK)
		writer.(http.Flusher).Flush()
		go func() {
			io.Copy(conn, request.Body)
			conn.(*net.TCPConn).CloseWrite()
		}()
		buffer := make([]byte, 4096)
		for {
			n, err := conn.Read(buffer)
			if n > 0 {
				_, err = writer.Write(buffer[:n])
				if err != nil {
					break
				}
				writer.(http.Flusher).Flush()
			}
			if err != nil {
				break
			}
		}
		conn.Close()
	})
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			server.connections.Add(1)
			go func() {
				err = conn.(*tls.Conn).Handshake()
				if err != nil {
					conn.Close()
					return
				}
				h2Server.ServeConn(conn, &http2.ServeConnOpts{Handler: handler})
			}()
		}
	}()
	t.Cleanup(func() {
		listener.Close()
	})
	return server
}

func (s *http2ProxyServer) port() uint16 {
	return uint16(s.listener.Addr().(*net.TCPAddr).Port)
}

func TestHTTPOutboundHTTP2(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	proxyServer := startHTTP2ProxyServer(t, certPem, keyPem)
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
						ServerPort: proxyServer.port(),
					},
					Username: "sekai",
					Password: "password",
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
	origin := newForwardOrigin(t)
	for i := 0; i < 3; i++ {
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
		client.CloseIdleConnections()
	}
	require.Equal(t, int32(1), proxyServer.connections.Load())
	require.Equal(t, int32(3), proxyServer.streams.Load())
}

func TestHTTPSelfTLS(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startTLSHTTPInbound(t, certPem, keyPem, nil, nil)
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
	require.Equal(t, int32(1), origin.connections.Load())
}

func TestHTTPForwardEarlyResponse(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startTLSHTTPInbound(t, certPem, keyPem, nil, nil)
	origin := startEarlyResponseOrigin(t)
	clientConn := dialHTTP2Proxy(t, serverPort)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+origin.String()+"/upload", io.LimitReader(rand.Reader, 4<<20))
	require.NoError(t, err)
	request.ContentLength = 4 << 20
	request.Header.Set("Proxy-Authorization", proxyAuthorization)
	response, err := clientConn.RoundTrip(request)
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, "ok", string(body))
}

func TestHTTPInboundHTTP2Cleartext(t *testing.T) {
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeHTTP,
				Options: &option.HTTPInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					Version: []int{2},
					Users:   []auth.User{{Username: "sekai", Password: "password"}},
				},
			},
		},
		Outbounds: []option.Outbound{{Type: C.TypeDirect}},
	})
	origin := newForwardOrigin(t)
	http1Conn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(int(serverPort)))
	require.NoError(t, err)
	defer http1Conn.Close()
	_, err = http1Conn.Write([]byte("CONNECT " + origin.host() + " HTTP/1.1\r\nHost: " + origin.host() + "\r\nProxy-Authorization: " + proxyAuthorization + "\r\n\r\n"))
	require.NoError(t, err)
	http1Response, err := http.ReadResponse(std_bufio.NewReader(http1Conn), nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusHTTPVersionNotSupported, http1Response.StatusCode)
	require.True(t, http1Response.Close)

	conn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(int(serverPort)))
	require.NoError(t, err)
	clientConn, err := (&http2.Transport{AllowHTTP: true}).NewClientConn(conn)
	require.NoError(t, err)
	defer clientConn.Close()
	tunnel := openHTTP2Tunnel(t, clientConn, origin.host(), proxyAuthorization)
	require.Equal(t, http.StatusOK, tunnel.response.StatusCode)
	_, err = tunnel.writer.Write([]byte("GET /hello HTTP/1.1\r\nHost: " + origin.host() + "\r\n\r\n"))
	require.NoError(t, err)
	response, err := http.ReadResponse(std_bufio.NewReader(tunnel.response.Body), nil)
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, "hello", string(body))
	tunnel.close()

	forwardRequest, err := http.NewRequest(http.MethodGet, origin.url("/hello"), nil)
	require.NoError(t, err)
	forwardRequest.Header.Set("Proxy-Authorization", proxyAuthorization)
	forwardResponse, err := clientConn.RoundTrip(forwardRequest)
	require.NoError(t, err)
	forwardResponse.Body.Close()
	require.Equal(t, http.StatusBadRequest, forwardResponse.StatusCode)
}

func TestHTTPOutboundHTTP2Deadline(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startTLSHTTPInbound(t, certPem, keyPem, nil, nil)
	origin := newForwardOrigin(t)
	detour, err := sTLS.NewDialerFromOptions(globalCtx, log.NewNOPFactory().Logger(), N.SystemDialer, "127.0.0.1", option.OutboundTLSOptions{
		Enabled:         true,
		ServerName:      "example.org",
		CertificatePath: certPem,
		ALPN:            []string{http2.NextProtoTLS},
	})
	require.NoError(t, err)
	client, err := sHTTP.NewClient(sHTTP.ClientOptions{
		Dialer:   detour,
		Server:   M.ParseSocksaddrHostPort("127.0.0.1", serverPort),
		Username: "sekai",
		Password: "password",
		Version:  2,
	})
	require.NoError(t, err)
	defer client.Close()
	conn, err := client.DialContext(context.Background(), N.NetworkTCP, M.ParseSocksaddr(origin.host()))
	require.NoError(t, err)
	defer conn.Close()
	require.NoError(t, conn.SetDeadline(time.Now().Add(100*time.Millisecond)))
	_, err = conn.Read(make([]byte, 1))
	require.True(t, E.IsTimeout(err))
	require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))
	_, err = conn.Write([]byte("GET /hello HTTP/1.1\r\nHost: " + origin.host() + "\r\n\r\n"))
	require.NoError(t, err)
	response, err := http.ReadResponse(std_bufio.NewReader(conn), nil)
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, "hello", string(body))
}

func TestHTTPOutboundHTTP2NoFallback(t *testing.T) {
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
					Version: []int{1},
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
					Version:                2,
					DisableVersionFallback: true,
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
	origin := newForwardOrigin(t)
	client := proxyClient(t, clientPort)
	for i := 0; i < 2; i++ {
		response, err := client.Get(origin.url("/hello"))
		require.NoError(t, err)
		response.Body.Close()
		require.Equal(t, http.StatusBadGateway, response.StatusCode)
	}
	require.Equal(t, int32(0), origin.connections.Load())
}

func startSwitchingProtocolsOrigin(t *testing.T) *net.TCPAddr {
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
				_, err := http.ReadRequest(std_bufio.NewReader(conn))
				if err != nil {
					return
				}
				conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n"))
			}()
		}
	}()
	return listener.Addr().(*net.TCPAddr)
}

func TestHTTPForwardUnexpectedSwitchingProtocols(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startTLSHTTPInbound(t, certPem, keyPem, nil, nil)
	origin := startSwitchingProtocolsOrigin(t)
	clientConn := dialHTTP2Proxy(t, serverPort)
	request, err := http.NewRequest(http.MethodGet, "http://"+origin.String()+"/", nil)
	require.NoError(t, err)
	request.Header.Set("Proxy-Authorization", proxyAuthorization)
	response, err := clientConn.RoundTrip(request)
	require.NoError(t, err)
	response.Body.Close()
	require.Equal(t, http.StatusBadGateway, response.StatusCode)
}

func startTruncatedChunkedOrigin(t *testing.T) *net.TCPAddr {
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
				_, err := http.ReadRequest(std_bufio.NewReader(conn))
				if err != nil {
					return
				}
				conn.Write([]byte("HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n5\r\nhello\r\n"))
			}()
		}
	}()
	return listener.Addr().(*net.TCPAddr)
}

func TestHTTPForwardHTTP2Responses(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startTLSHTTPInbound(t, certPem, keyPem, nil, nil)
	clientConn := dialHTTP2Proxy(t, serverPort)

	origin := newForwardOrigin(t)
	headRequest, err := http.NewRequest(http.MethodHead, origin.url("/hello"), nil)
	require.NoError(t, err)
	headRequest.Header.Set("User-Agent", "")
	headRequest.Header.Set("Proxy-Authorization", proxyAuthorization)
	headResponse, err := clientConn.RoundTrip(headRequest)
	require.NoError(t, err)
	headResponse.Body.Close()
	require.Equal(t, http.StatusOK, headResponse.StatusCode)
	require.Equal(t, int64(5), headResponse.ContentLength)

	trailerRequest, err := http.NewRequest(http.MethodGet, origin.url("/trailer"), nil)
	require.NoError(t, err)
	trailerRequest.Header.Set("Proxy-Authorization", proxyAuthorization)
	trailerResponse, err := clientConn.RoundTrip(trailerRequest)
	require.NoError(t, err)
	trailerBody, err := io.ReadAll(trailerResponse.Body)
	require.NoError(t, err)
	trailerResponse.Body.Close()
	require.Equal(t, "hello", string(trailerBody))
	require.Equal(t, "5d41402a", trailerResponse.Trailer.Get("X-Checksum"))

	streamRequest, err := http.NewRequest(http.MethodGet, origin.url("/stream"), nil)
	require.NoError(t, err)
	streamRequest.Header.Set("Proxy-Authorization", proxyAuthorization)
	startTime := time.Now()
	streamResponse, err := clientConn.RoundTrip(streamRequest)
	require.NoError(t, err)
	require.Less(t, time.Since(startTime), time.Second)
	streamBody, err := io.ReadAll(streamResponse.Body)
	streamResponse.Body.Close()
	require.NoError(t, err)
	require.Equal(t, "late", string(streamBody))

	truncated := startTruncatedChunkedOrigin(t)
	request, err := http.NewRequest(http.MethodGet, "http://"+truncated.String()+"/", nil)
	require.NoError(t, err)
	request.Header.Set("Proxy-Authorization", proxyAuthorization)
	response, err := clientConn.RoundTrip(request)
	require.NoError(t, err)
	_, err = io.ReadAll(response.Body)
	response.Body.Close()
	require.Error(t, err)
}
