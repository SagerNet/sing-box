package v2raygrpclite

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.V2RayClientTransport = (*Client)(nil)

var defaultClientHeader = http.Header{
	"Content-Type": []string{"application/grpc"},
	"User-Agent":   []string{"grpc-go/1.48.0"},
	"TE":           []string{"trailers"},
}

type Client struct {
	ctx        context.Context
	dialer     N.Dialer
	serverAddr M.Socksaddr
	transport  *http.Transport
	options    option.V2RayGRPCOptions
	url        *url.URL
}

func NewClient(ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr, options option.V2RayGRPCOptions, tlsConfig *tls.Config) adapter.V2RayClientTransport {
	return &Client{
		ctx:        ctx,
		dialer:     dialer,
		serverAddr: serverAddr,
		options:    options,
		transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
			},
			ForceAttemptHTTP2: true,
			TLSClientConfig:   tlsConfig,
		},
		url: &url.URL{
			Scheme: "https",
			Host:   serverAddr.String(),
			Path:   fmt.Sprintf("/%s/Tun", url.QueryEscape(options.ServiceName)),
		},
	}
}

func (c *Client) DialContext(ctx context.Context) (net.Conn, error) {
	pipeInReader, pipeInWriter := io.Pipe()
	request := &http.Request{
		Method:     http.MethodPost,
		Body:       pipeInReader,
		URL:        c.url,
		Proto:      "HTTP/2",
		ProtoMajor: 2,
		Header:     defaultClientHeader,
	}
	request = request.WithContext(ctx)
	conn := newLateGunConn(pipeInWriter)
	go func() {
		response, err := c.transport.RoundTrip(request)
		if err == nil {
			conn.setup(response.Body, nil)
		} else {
			conn.setup(nil, err)
		}
	}()
	return conn, nil
}
