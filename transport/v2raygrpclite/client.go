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
	D "github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"golang.org/x/net/http2"
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
	client     *http.Client
	options    option.V2RayGRPCOptions
	url        *url.URL
}

func NewClient(ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr, options option.V2RayGRPCOptions, tlsConfig *tls.Config) adapter.V2RayClientTransport {
	return &Client{
		ctx:        ctx,
		dialer:     dialer,
		serverAddr: serverAddr,
		options:    options,
		client: &http.Client{
			Transport: &http2.Transport{
				DialTLSContext: func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
					conn, err := dialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
					if err != nil {
						return nil, err
					}
					tlsConn, err := D.TLSClient(ctx, conn, cfg)
					if err != nil {
						return nil, err
					}
					return tlsConn, nil
				},
				TLSClientConfig:    tlsConfig,
				AllowHTTP:          false,
				DisableCompression: true,
				PingTimeout:        0,
			},
		},
		url: &url.URL{
			Scheme: "https",
			Host:   serverAddr.String(),
			Path:   fmt.Sprintf("/%s/Tun", url.QueryEscape(options.ServiceName)),
		},
	}
}

func (c *Client) DialContext(ctx context.Context) (net.Conn, error) {
	requestPipeReader, requestPipeWriter := io.Pipe()
	request := (&http.Request{
		Method:     http.MethodPost,
		Body:       requestPipeReader,
		URL:        c.url,
		Proto:      "HTTP/2",
		ProtoMajor: 2,
		ProtoMinor: 0,
		Header:     defaultClientHeader,
	}).WithContext(ctx)
	responsePipeReader, responsePipeWriter := io.Pipe()
	go func() {
		defer responsePipeWriter.Close()
		response, err := c.client.Do(request)
		if err != nil {
			return
		}
		bufio.Copy(responsePipeWriter, response.Body)
	}()
	return newGunConn(responsePipeReader, requestPipeWriter, ChainedClosable{requestPipeReader, requestPipeWriter, responsePipeReader}), nil
}

type ChainedClosable []io.Closer

// Close implements io.Closer.Close().
func (cc ChainedClosable) Close() error {
	for _, c := range cc {
		_ = common.Close(c)
	}
	return nil
}
