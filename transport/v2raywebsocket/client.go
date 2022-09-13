package v2raywebsocket

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/websocket"
)

var _ adapter.V2RayClientTransport = (*Client)(nil)

type Client struct {
	dialer              *websocket.Dialer
	uri                 string
	headers             http.Header
	maxEarlyData        uint32
	earlyDataHeaderName string
}

func NewClient(ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr, options option.V2RayWebsocketOptions, tlsConfig *tls.Config) adapter.V2RayClientTransport {
	wsDialer := &websocket.Dialer{
		NetDialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
		},
		TLSClientConfig:  tlsConfig,
		ReadBufferSize:   4 * 1024,
		WriteBufferSize:  4 * 1024,
		HandshakeTimeout: time.Second * 8,
	}
	var uri url.URL
	if tlsConfig == nil {
		uri.Scheme = "ws"
	} else {
		uri.Scheme = "wss"
	}
	uri.Host = serverAddr.String()
	uri.Path = options.Path
	if !strings.HasPrefix(uri.Path, "/") {
		uri.Path = "/" + uri.Path
	}
	headers := make(http.Header)
	for key, value := range options.Headers {
		headers.Set(key, value)
	}
	return &Client{
		wsDialer,
		uri.String(),
		headers,
		options.MaxEarlyData,
		options.EarlyDataHeaderName,
	}
}

func (c *Client) DialContext(ctx context.Context) (net.Conn, error) {
	if c.maxEarlyData <= 0 {
		conn, response, err := c.dialer.DialContext(ctx, c.uri, c.headers)
		if err == nil {
			return &WebsocketConn{Conn: conn}, nil
		}
		return nil, wrapDialError(response, err)
	} else {
		return &EarlyWebsocketConn{Client: c, ctx: ctx, create: make(chan struct{})}, nil
	}
}

func wrapDialError(response *http.Response, err error) error {
	if response == nil {
		return err
	}
	return E.Extend(err, "HTTP ", response.StatusCode, " ", response.Status)
}
