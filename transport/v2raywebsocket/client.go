package v2raywebsocket

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	sHTTP "github.com/sagernet/sing/protocol/http"
	"github.com/sagernet/websocket"
)

var _ adapter.V2RayClientTransport = (*Client)(nil)

type Client struct {
	dialer              *websocket.Dialer
	requestURL          url.URL
	requestURLString    string
	headers             http.Header
	maxEarlyData        uint32
	earlyDataHeaderName string
}

func NewClient(ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr, options option.V2RayWebsocketOptions, tlsConfig tls.Config) adapter.V2RayClientTransport {
	wsDialer := &websocket.Dialer{
		ReadBufferSize:   4 * 1024,
		WriteBufferSize:  4 * 1024,
		HandshakeTimeout: time.Second * 8,
	}
	if tlsConfig != nil {
		if len(tlsConfig.NextProtos()) == 0 {
			tlsConfig.SetNextProtos([]string{"http/1.1"})
		}
		wsDialer.NetDialTLSContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := dialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
			if err != nil {
				return nil, err
			}
			tlsConn, err := tls.ClientHandshake(ctx, conn, tlsConfig)
			if err != nil {
				return nil, err
			}
			return &deadConn{tlsConn}, nil
		}
	} else {
		wsDialer.NetDialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := dialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
			if err != nil {
				return nil, err
			}
			return &deadConn{conn}, nil
		}
	}
	var requestURL url.URL
	if tlsConfig == nil {
		requestURL.Scheme = "ws"
	} else {
		requestURL.Scheme = "wss"
	}
	requestURL.Host = serverAddr.String()
	requestURL.Path = options.Path
	err := sHTTP.URLSetPath(&requestURL, options.Path)
	if err != nil {
		return nil
	}
	headers := make(http.Header)
	for key, value := range options.Headers {
		headers[key] = value
	}
	return &Client{
		wsDialer,
		requestURL,
		requestURL.String(),
		headers,
		options.MaxEarlyData,
		options.EarlyDataHeaderName,
	}
}

func (c *Client) DialContext(ctx context.Context) (net.Conn, error) {
	if c.maxEarlyData <= 0 {
		conn, response, err := c.dialer.DialContext(ctx, c.requestURLString, c.headers)
		if err == nil {
			return &WebsocketConn{Conn: conn, Writer: NewWriter(conn, false)}, nil
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
