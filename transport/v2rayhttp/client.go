package v2rayhttp

import (
	"context"
	"crypto/tls"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.V2RayClientTransport = (*Client)(nil)

type Client struct {
	ctx     context.Context
	client  *http.Client
	url     *url.URL
	host    []string
	method  string
	headers http.Header
}

func NewClient(ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr, options option.V2RayHTTPOptions, tlsConfig *tls.Config) adapter.V2RayClientTransport {
	client := &Client{
		ctx:     ctx,
		host:    options.Host,
		method:  options.Method,
		headers: make(http.Header),
		client: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return dialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
				},
				ForceAttemptHTTP2: true,
				TLSClientConfig:   tlsConfig,
			},
		},
	}
	if client.method == "" {
		client.method = "PUT"
	}
	var uri url.URL
	if tlsConfig == nil {
		uri.Scheme = "http"
	} else {
		uri.Scheme = "https"
	}
	uri.Host = serverAddr.String()
	uri.Path = options.Path
	if !strings.HasPrefix(uri.Path, "/") {
		uri.Path = "/" + uri.Path
	}
	for key, value := range options.Headers {
		client.headers.Set(key, value)
	}
	client.url = &uri
	return client
}

func (c *Client) DialContext(ctx context.Context) (net.Conn, error) {
	pipeInReader, pipeInWriter := io.Pipe()
	request := &http.Request{
		Method:     c.method,
		Body:       pipeInReader,
		URL:        c.url,
		ProtoMajor: 2,
		ProtoMinor: 0,
		Proto:      "HTTP/2",
		Header:     c.headers.Clone(),
	}
	switch hostLen := len(c.host); hostLen {
	case 0:
	case 1:
		request.Host = c.host[0]
	default:
		request.Host = c.host[rand.Intn(hostLen)]
	}
	// Disable any compression method from server.
	request.Header.Set("Accept-Encoding", "identity")
	response, err := c.client.Do(request) // nolint: bodyclose
	if err != nil {
		pipeInWriter.Close()
		return nil, err
	}
	if response.StatusCode != 200 {
		return nil, E.New("unexpected status: ", response.StatusCode, " ", response.Status)
	}
	return &HTTPConn{
		response.Body,
		pipeInWriter,
	}, nil
}
