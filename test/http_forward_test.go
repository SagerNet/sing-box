package main

import (
	std_bufio "bufio"
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"net/netip"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/json/badoption"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

type forwardOrigin struct {
	*httptest.Server
	connections atomic.Int32
}

func newForwardOrigin(t *testing.T) *forwardOrigin {
	origin := &forwardOrigin{}
	mux := http.NewServeMux()
	mux.HandleFunc("/hello", func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Proxy-Authorization") != "" || request.Header.Get("Proxy-Connection") != "" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		if _, loaded := request.Header["User-Agent"]; loaded {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		writer.Header().Set("X-Host", request.Host)
		writer.Write([]byte("hello"))
	})
	mux.HandleFunc("/echo", func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		writer.Write(body)
	})
	mux.HandleFunc("/chunked", func(writer http.ResponseWriter, request *http.Request) {
		flusher := writer.(http.Flusher)
		for i := 0; i < 3; i++ {
			writer.Write([]byte("chunk"))
			flusher.Flush()
		}
	})
	mux.HandleFunc("/trailer", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Trailer", "X-Checksum")
		writer.Write([]byte("hello"))
		writer.Header().Set("X-Checksum", "5d41402a")
	})
	mux.HandleFunc("/stream", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		writer.(http.Flusher).Flush()
		time.Sleep(2 * time.Second)
		writer.Write([]byte("late"))
	})
	mux.HandleFunc("/hints", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Link", "</style.css>; rel=preload")
		writer.WriteHeader(http.StatusEarlyHints)
		writer.Write([]byte("body"))
	})
	mux.HandleFunc("/ws", func(writer http.ResponseWriter, request *http.Request) {
		conn, err := websocket.Accept(writer, request, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		messageType, message, err := conn.Read(request.Context())
		if err != nil {
			return
		}
		conn.Write(request.Context(), messageType, message)
		conn.Close(websocket.StatusNormalClosure, "")
	})
	origin.Server = httptest.NewUnstartedServer(mux)
	origin.Config.ConnState = func(conn net.Conn, state http.ConnState) {
		if state == http.StateNew {
			origin.connections.Add(1)
		}
	}
	origin.Start()
	t.Cleanup(origin.Close)
	return origin
}

func (o *forwardOrigin) url(path string) string {
	return o.URL + path
}

func (o *forwardOrigin) host() string {
	return strings.TrimPrefix(o.URL, "http://")
}

func startForwardProxy(t *testing.T) {
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
}

func proxyClient(t *testing.T, port uint16) *http.Client {
	proxyURL, err := url.Parse("http://127.0.0.1:" + strconv.Itoa(int(port)))
	require.NoError(t, err)
	client := &http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 10 * time.Second,
	}
	t.Cleanup(client.CloseIdleConnections)
	return client
}

func TestHTTPForwardKeepAlive(t *testing.T) {
	startForwardProxy(t)
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
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.Equal(t, "hello", string(body))
		require.Equal(t, origin.host(), response.Header.Get("X-Host"))
	}
	response, err := client.Post(origin.url("/echo"), "application/octet-stream", strings.NewReader(strings.Repeat("payload", 1024)))
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	require.NoError(t, err)
	require.Equal(t, strings.Repeat("payload", 1024), string(body))
	request, err := http.NewRequest(http.MethodHead, origin.url("/hello"), nil)
	require.NoError(t, err)
	request.Header.Set("User-Agent", "")
	response, err = client.Do(request)
	require.NoError(t, err)
	response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, int64(5), response.ContentLength)
	require.Equal(t, int32(1), origin.connections.Load())
}

func TestHTTPForwardChunked(t *testing.T) {
	startForwardProxy(t)
	origin := newForwardOrigin(t)
	client := proxyClient(t, clientPort)
	response, err := client.Get(origin.url("/chunked"))
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	require.NoError(t, err)
	require.Equal(t, "chunkchunkchunk", string(body))
	require.Equal(t, []string{"chunked"}, response.TransferEncoding)
	response, err = client.Get(origin.url("/hello"))
	require.NoError(t, err)
	response.Body.Close()
	require.Equal(t, int32(1), origin.connections.Load())
}

func TestHTTPForwardEarlyHints(t *testing.T) {
	startForwardProxy(t)
	origin := newForwardOrigin(t)
	client := proxyClient(t, clientPort)
	var hints atomic.Int32
	trace := &httptrace.ClientTrace{
		Got1xxResponse: func(code int, header textproto.MIMEHeader) error {
			if code == http.StatusEarlyHints && header.Get("Link") != "" {
				hints.Add(1)
			}
			return nil
		},
	}
	request, err := http.NewRequestWithContext(httptrace.WithClientTrace(context.Background(), trace), http.MethodGet, origin.url("/hints"), nil)
	require.NoError(t, err)
	response, err := client.Do(request)
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	require.NoError(t, err)
	require.Equal(t, "body", string(body))
	require.Equal(t, int32(1), hints.Load())

	conn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(int(serverPort)))
	require.NoError(t, err)
	defer conn.Close()
	_, err = conn.Write([]byte("GET " + origin.url("/hints") + " HTTP/1.1\r\nHost: " + origin.host() + "\r\nProxy-Authorization: " + proxyAuthorization + "\r\n\r\n"))
	require.NoError(t, err)
	reader := std_bufio.NewReader(conn)
	interim, err := http.ReadResponse(reader, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusEarlyHints, interim.StatusCode)
	require.Empty(t, interim.Header.Values("Content-Length"))
	require.Empty(t, interim.Header.Values("Transfer-Encoding"))
	final, err := http.ReadResponse(reader, nil)
	require.NoError(t, err)
	body, err = io.ReadAll(final.Body)
	require.NoError(t, err)
	require.Equal(t, "body", string(body))
}

func TestHTTPForwardWebSocket(t *testing.T) {
	startForwardProxy(t)
	origin := newForwardOrigin(t)
	client := proxyClient(t, clientPort)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws://"+origin.host()+"/ws", &websocket.DialOptions{HTTPClient: client})
	require.NoError(t, err)
	defer conn.CloseNow()
	err = conn.Write(ctx, websocket.MessageText, []byte("ping"))
	require.NoError(t, err)
	messageType, message, err := conn.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, websocket.MessageText, messageType)
	require.Equal(t, "ping", string(message))
}

func TestHTTPForwardConnect(t *testing.T) {
	startForwardProxy(t)
	origin := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte("secure"))
	}))
	defer origin.Close()
	client := proxyClient(t, clientPort)
	response, err := client.Get(origin.URL)
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	require.NoError(t, err)
	require.Equal(t, "secure", string(body))
}

func TestHTTPForwardLegacyClient(t *testing.T) {
	startForwardProxy(t)
	origin := newForwardOrigin(t)
	conn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(int(clientPort)))
	require.NoError(t, err)
	defer conn.Close()
	reader := std_bufio.NewReader(conn)
	for i := 0; i < 2; i++ {
		_, err = conn.Write([]byte("GET " + origin.url("/hello") + " HTTP/1.0\r\nHost: " + origin.host() + "\r\nUser-Agent: \r\nProxy-Connection: keep-alive\r\n\r\n"))
		require.NoError(t, err)
		response, err := http.ReadResponse(reader, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.Equal(t, int64(5), response.ContentLength)
		require.Equal(t, "keep-alive", response.Header.Get("Connection"))
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		require.NoError(t, err)
		require.Equal(t, "hello", string(body))
	}
	_, err = conn.Write([]byte("GET " + origin.url("/chunked") + " HTTP/1.0\r\nHost: " + origin.host() + "\r\nProxy-Connection: keep-alive\r\n\r\n"))
	require.NoError(t, err)
	response, err := http.ReadResponse(reader, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Empty(t, response.TransferEncoding)
	require.Equal(t, int64(-1), response.ContentLength)
	require.True(t, response.Close)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, "chunkchunkchunk", string(body))
	_, err = reader.ReadByte()
	require.ErrorIs(t, err, io.EOF)
	require.Equal(t, int32(1), origin.connections.Load())
}

func TestHTTPForwardAuthRetry(t *testing.T) {
	startForwardProxy(t)
	origin := newForwardOrigin(t)
	conn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(int(serverPort)))
	require.NoError(t, err)
	defer conn.Close()
	reader := std_bufio.NewReader(conn)
	_, err = conn.Write([]byte("GET " + origin.url("/hello") + " HTTP/1.1\r\nHost: " + origin.host() + "\r\nUser-Agent: \r\n\r\n"))
	require.NoError(t, err)
	response, err := http.ReadResponse(reader, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusProxyAuthRequired, response.StatusCode)
	require.Contains(t, response.Header.Get("Proxy-Authenticate"), "Basic")
	require.False(t, response.Close)
	_, err = io.ReadAll(response.Body)
	require.NoError(t, err)
	response.Body.Close()
	request, err := http.NewRequest(http.MethodGet, origin.url("/hello"), nil)
	require.NoError(t, err)
	request.Header.Set("User-Agent", "")
	request.Header.Set("Proxy-Authorization", "Basic c2VrYWk6cGFzc3dvcmQ=")
	err = request.WriteProxy(conn)
	require.NoError(t, err)
	response, err = http.ReadResponse(reader, request)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	require.NoError(t, err)
	require.Equal(t, "hello", string(body))
	_, err = conn.Write([]byte("CONNECT " + origin.host() + " HTTP/1.1\r\nHost: " + origin.host() + "\r\nProxy-Authorization: Basic c2VrYWk6cGFzc3dvcmQ=\r\n\r\n"))
	require.NoError(t, err)
	response, err = http.ReadResponse(reader, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	request, err = http.NewRequest(http.MethodGet, origin.url("/hello"), nil)
	require.NoError(t, err)
	request.Header.Set("User-Agent", "")
	err = request.Write(conn)
	require.NoError(t, err)
	response, err = http.ReadResponse(reader, request)
	require.NoError(t, err)
	body, err = io.ReadAll(response.Body)
	response.Body.Close()
	require.NoError(t, err)
	require.Equal(t, "hello", string(body))
}

func TestHTTPForwardCloseOnError(t *testing.T) {
	startForwardProxy(t)
	conn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(int(serverPort)))
	require.NoError(t, err)
	defer conn.Close()
	_, err = conn.Write([]byte("GET http://127.0.0.1:1/ HTTP/1.1\r\nHost: 127.0.0.1:1\r\nConnection: close\r\nProxy-Authorization: " + proxyAuthorization + "\r\n\r\n"))
	require.NoError(t, err)
	reader := std_bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadGateway, response.StatusCode)
	require.True(t, response.Close)
	_, err = io.ReadAll(response.Body)
	require.NoError(t, err)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, err = reader.ReadByte()
	require.ErrorIs(t, err, io.EOF)
}

func TestHTTPForwardUserIsolation(t *testing.T) {
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeHTTP,
				Options: &option.HTTPInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					Users: []auth.User{
						{Username: "sekai", Password: "password"},
						{Username: "blocked", Password: "password"},
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{Type: C.TypeDirect},
			{Type: C.TypeBlock, Tag: "block"},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RawDefaultRule: option.RawDefaultRule{
							AuthUser: []string{"blocked"},
						},
						RuleAction: option.RuleAction{
							Action:       C.RuleActionTypeRoute,
							RouteOptions: option.RouteActionOptions{Outbound: "block"},
						},
					},
				},
			},
		},
	})
	origin := newForwardOrigin(t)
	conn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(int(serverPort)))
	require.NoError(t, err)
	defer conn.Close()
	reader := std_bufio.NewReader(conn)
	for _, authorization := range []string{proxyAuthorization, "Basic YmxvY2tlZDpwYXNzd29yZA==", proxyAuthorization} {
		_, err = conn.Write([]byte("GET " + origin.url("/hello") + " HTTP/1.1\r\nHost: " + origin.host() + "\r\nUser-Agent: \r\nProxy-Authorization: " + authorization + "\r\n\r\n"))
		require.NoError(t, err)
		response, err := http.ReadResponse(reader, nil)
		require.NoError(t, err)
		_, err = io.ReadAll(response.Body)
		require.NoError(t, err)
		response.Body.Close()
		if authorization == proxyAuthorization {
			require.Equal(t, http.StatusOK, response.StatusCode)
		} else {
			require.Equal(t, http.StatusBadGateway, response.StatusCode)
		}
		require.False(t, response.Close)
	}
	require.Equal(t, int32(2), origin.connections.Load())
}

func startEarlyResponseOrigin(t *testing.T) *net.TCPAddr {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	stop := make(chan struct{})
	t.Cleanup(func() {
		close(stop)
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
				_, err := reader.ReadString('\n')
				if err == nil {
					_, err = textproto.NewReader(reader).ReadMIMEHeader()
				}
				if err != nil {
					return
				}
				_, err = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok"))
				if err != nil {
					return
				}
				<-stop
			}()
		}
	}()
	return listener.Addr().(*net.TCPAddr)
}

func startHugeHeaderOrigin(t *testing.T) *net.TCPAddr {
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
				_, err = conn.Write([]byte("HTTP/1.1 200 OK\r\nX-Padding: "))
				if err != nil {
					return
				}
				padding := bytes.Repeat([]byte("a"), 64<<10)
				for {
					_, err = conn.Write(padding)
					if err != nil {
						return
					}
				}
			}()
		}
	}()
	return listener.Addr().(*net.TCPAddr)
}

func TestHTTPForwardHugeUpstreamHeader(t *testing.T) {
	startForwardProxy(t)
	origin := startHugeHeaderOrigin(t)
	client := proxyClient(t, clientPort)
	response, err := client.Get("http://" + origin.String() + "/")
	require.NoError(t, err)
	response.Body.Close()
	require.Equal(t, http.StatusBadGateway, response.StatusCode)
}
