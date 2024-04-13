package v2rayhttpupgrade

import (
	std_bufio "bufio"
	"context"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/atomic"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	sHTTP "github.com/sagernet/sing/protocol/http"
)

var _ adapter.V2RayClientTransport = (*Client)(nil)

type Client struct {
	dialer     N.Dialer
	tlsConfig  tls.Config
	serverAddr M.Socksaddr
	requestURL url.URL
	headers    http.Header
	host       string
	fastOpen   bool
}

func NewClient(ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr, options option.V2RayHTTPUpgradeOptions, tlsConfig tls.Config) (*Client, error) {
	if tlsConfig != nil {
		if len(tlsConfig.NextProtos()) == 0 {
			tlsConfig.SetNextProtos([]string{"http/1.1"})
		}
	}
	var host string
	if options.Host != "" {
		host = options.Host
	} else if tlsConfig != nil && tlsConfig.ServerName() != "" {
		host = tlsConfig.ServerName()
	} else {
		host = serverAddr.String()
	}
	var requestURL url.URL
	if tlsConfig == nil {
		requestURL.Scheme = "http"
	} else {
		requestURL.Scheme = "https"
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
	}
	return &Client{
		dialer:     dialer,
		tlsConfig:  tlsConfig,
		serverAddr: serverAddr,
		requestURL: requestURL,
		headers:    headers,
		host:       host,
		fastOpen:   options.FastOpen,
	}, nil
}

func (c *Client) DialContext(ctx context.Context) (net.Conn, error) {
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
	request := &http.Request{
		Method: http.MethodGet,
		URL:    &c.requestURL,
		Header: c.headers.Clone(),
		Host:   c.host,
	}
	request.Header.Set("Connection", "Upgrade")
	request.Header.Set("Upgrade", "websocket")
	err = request.Write(conn)
	if err != nil {
		return nil, err
	}
	if c.fastOpen {
		return &EarlyHTTPUpgradeConn{
			Conn: conn,
		}, nil
	}
	return readResponse(conn)
}

func readResponse(conn net.Conn) (net.Conn, error) {
	bufReader := std_bufio.NewReader(conn)
	response, err := http.ReadResponse(bufReader, nil)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != 101 ||
		!strings.EqualFold(response.Header.Get("Connection"), "upgrade") ||
		!strings.EqualFold(response.Header.Get("Upgrade"), "websocket") {
		return nil, E.New("unexpected status: ", response.Status)
	}
	if bufReader.Buffered() > 0 {
		buffer := buf.NewSize(bufReader.Buffered())
		_, err = buffer.ReadFullFrom(bufReader, buffer.Len())
		if err != nil {
			return nil, err
		}
		conn = bufio.NewCachedConn(conn, buffer)
	}
	return conn, nil
}

type EarlyHTTPUpgradeConn struct {
	net.Conn
	once    sync.Once
	err     error
	created atomic.Bool
}

func (c *EarlyHTTPUpgradeConn) Read(b []byte) (int, error) {
	c.once.Do(func() {
		var newConn net.Conn
		newConn, c.err = readResponse(c.Conn)
		if c.err == nil {
			c.Conn = newConn
			c.created.Store(true)
		}
	})
	if c.err != nil {
		return 0, c.err
	}
	return c.Conn.Read(b)
}

func (c *EarlyHTTPUpgradeConn) Upstream() any {
	return c.Conn
}

func (c *EarlyHTTPUpgradeConn) ReaderReplaceable() bool {
	return c.created.Load()
}

func (c *EarlyHTTPUpgradeConn) WriterReplaceable() bool {
	return true
}

func (c *EarlyHTTPUpgradeConn) SetDeadline(time.Time) error {
	return os.ErrInvalid
}

func (c *EarlyHTTPUpgradeConn) SetReadDeadline(time.Time) error {
	return os.ErrInvalid
}

func (c *EarlyHTTPUpgradeConn) SetWriteDeadline(time.Time) error {
	return os.ErrInvalid
}

func (c *EarlyHTTPUpgradeConn) NeedAdditionalReadDeadline() bool {
	return true
}
