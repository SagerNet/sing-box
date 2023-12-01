package v2raywebsocket

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/bufio/deadline"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	sHTTP "github.com/sagernet/sing/protocol/http"
	"github.com/sagernet/ws"
)

var _ adapter.V2RayClientTransport = (*Client)(nil)

type Client struct {
	dialer              N.Dialer
	tlsConfig           tls.Config
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
		return nil, E.Cause(err, "parse path")
	}
	if !strings.HasPrefix(requestURL.Path, "/") {
		requestURL.Path = "/" + requestURL.Path
	}
	headers := make(http.Header)
	for key, value := range options.Headers {
		headers[key] = value
		if key == "Host" {
			if len(value) > 1 {
				return nil, E.New("multiple Host headers")
			}
			requestURL.Host = value[0]
		}
	}
	if headers.Get("User-Agent") == "" {
		headers.Set("User-Agent", "Go-http-client/1.1")
	}
	return &Client{
		dialer,
		tlsConfig,
		serverAddr,
		requestURL,
		headers,
		options.MaxEarlyData,
		options.EarlyDataHeaderName,
	}, nil
}

func (c *Client) dialContext(ctx context.Context, requestURL *url.URL, headers http.Header) (*WebsocketConn, error) {
	conn, err := c.dialer.DialContext(ctx, N.NetworkTCP, c.serverAddr)
	if err != nil {
		return nil, err
	}
	if c.tlsConfig != nil {
		conn, err = tls.ClientHandshake(ctx, conn, c.tlsConfig)
		if err != nil {
			return nil, err
		}
	}
	var deadlineConn net.Conn
	if deadline.NeedAdditionalReadDeadline(conn) {
		deadlineConn = deadline.NewConn(conn)
	} else {
		deadlineConn = conn
	}
	err = deadlineConn.SetDeadline(time.Now().Add(C.TCPTimeout))
	if err != nil {
		return nil, E.Cause(err, "set read deadline")
	}
	var protocols []string
	if protocolHeader := headers.Get("Sec-WebSocket-Protocol"); protocolHeader != "" {
		protocols = []string{protocolHeader}
		headers.Del("Sec-WebSocket-Protocol")
	}
	reader, _, err := ws.Dialer{Header: ws.HandshakeHeaderHTTP(headers), Protocols: protocols}.Upgrade(deadlineConn, requestURL)
	deadlineConn.SetDeadline(time.Time{})
	if err != nil {
		return nil, err
	}
	if reader != nil {
		buffer := buf.NewSize(reader.Buffered())
		_, err = buffer.ReadFullFrom(reader, buffer.Len())
		if err != nil {
			return nil, err
		}
		conn = bufio.NewCachedConn(conn, buffer)
	}
	return NewConn(conn, nil, ws.StateClientSide), nil
}

func (c *Client) DialContext(ctx context.Context) (net.Conn, error) {
	if c.maxEarlyData <= 0 {
		conn, err := c.dialContext(ctx, &c.requestURL, c.headers)
		if err != nil {
			return nil, err
		}
		return conn, nil
	} else {
		return &EarlyWebsocketConn{Client: c, ctx: ctx, create: make(chan struct{})}, nil
	}
}
