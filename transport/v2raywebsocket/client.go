package v2raywebsocket

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/badhttp"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/ws"
)

var _ adapter.V2RayClientTransport = (*Client)(nil)

type Client struct {
	dialer              N.Dialer
	serverAddr          M.Socksaddr
	requestURL          url.URL
	headers             http.Header
	maxEarlyData        uint32
	earlyDataHeaderName string
}

func NewClient(ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr, options option.V2RayWebsocketOptions, tlsConfig tls.Config) (adapter.V2RayClientTransport, error) {
	if tlsConfig != nil {
		if len(tlsConfig.NextProtos()) == 0 {
			tlsConfig.SetNextProtos([]string{"http/1.1"})
		}
		dialer = tls.NewDialer(dialer, tlsConfig)
	}
	var requestURL url.URL
	if tlsConfig == nil {
		requestURL.Scheme = "ws"
	} else {
		requestURL.Scheme = "wss"
	}
	requestURL.Host = serverAddr.String()
	requestURL.Path = options.Path
	err := badhttp.URLSetPath(&requestURL, options.Path)
	if err != nil {
		return nil, E.Cause(err, "parse path")
	}
	if !strings.HasPrefix(requestURL.Path, "/") {
		requestURL.Path = "/" + requestURL.Path
	}
	headers := options.Headers.Build()
	if host := headers.Get("Host"); host != "" {
		headers.Del("Host")
		requestURL.Host = host
	}
	if headers.Get("User-Agent") == "" {
		headers.Set("User-Agent", "Go-http-client/1.1")
	}
	return &Client{
		dialer,
		serverAddr,
		requestURL,
		headers,
		options.MaxEarlyData,
		options.EarlyDataHeaderName,
	}, nil
}

func (c *Client) DialContext(ctx context.Context) (net.Conn, error) {
	conn, err := c.dialer.DialContext(ctx, N.NetworkTCP, c.serverAddr)
	if err != nil {
		return nil, err
	}
	if c.maxEarlyData > 0 {
		return &EarlyWebsocketConn{Client: c, rawConn: conn, create: make(chan struct{})}, nil
	}
	websocketConn, err := c.upgrade(ctx, conn, &c.requestURL, c.headers)
	if err != nil {
		return nil, err
	}
	return websocketConn, nil
}

func (c *Client) upgrade(ctx context.Context, conn net.Conn, requestURL *url.URL, headers http.Header) (*WebsocketConn, error) {
	var protocols []string
	protocolHeader := headers.Get("Sec-WebSocket-Protocol")
	if protocolHeader != "" {
		protocols = []string{protocolHeader}
		headers = headers.Clone()
		headers.Del("Sec-WebSocket-Protocol")
	}
	ctx, cancel := context.WithTimeout(ctx, C.TCPTimeout)
	defer cancel()
	stop := context.AfterFunc(ctx, func() {
		conn.Close()
	})
	reader, _, err := ws.Dialer{Header: ws.HandshakeHeaderHTTP(headers), Protocols: protocols}.Upgrade(conn, requestURL)
	if !stop() {
		err = ctx.Err()
	}
	if err != nil {
		conn.Close()
		return nil, err
	}
	if reader != nil {
		buffer := buf.NewSize(reader.Buffered())
		_, err = buffer.ReadFullFrom(reader, buffer.FreeLen())
		if err != nil {
			conn.Close()
			return nil, err
		}
		conn = bufio.NewCachedConn(conn, buffer)
	}
	return NewConn(conn, nil, ws.StateClientSide), nil
}

func (c *Client) Close() error {
	return nil
}
