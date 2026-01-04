package v2rayxhttp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	sHTTP "github.com/sagernet/sing/protocol/http"

	"golang.org/x/net/http2"
)

var _ adapter.V2RayClientTransport = (*Client)(nil)

type Client struct {
	ctx                  context.Context
	dialer               N.Dialer
	serverAddr           M.Socksaddr
	transport            http.RoundTripper
	requestURL           url.URL
	host                 string
	headers              http.Header
	mode                 string
	noGRPCHeader         bool
	noSSEHeader          bool
	xPaddingBytes        *option.RangeConfig
	scMaxEachPostBytes   *option.RangeConfig
	scMinPostsIntervalMs *option.RangeConfig
	http2                bool
}

func NewClient(ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr, options option.V2RayXHTTPOptions, tlsConfig tls.Config) (adapter.V2RayClientTransport, error) {
	var transport http.RoundTripper
	if tlsConfig == nil {
		transport = &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
			},
		}
	} else {
		if len(tlsConfig.NextProtos()) == 0 {
			tlsConfig.SetNextProtos([]string{http2.NextProtoTLS})
		}
		tlsDialer := tls.NewDialer(dialer, tlsConfig)
		transport = &http2.Transport{
			ReadIdleTimeout: 30 * time.Second,
			PingTimeout:     15 * time.Second,
			DialTLSContext: func(ctx context.Context, network, addr string, cfg *tls.STDConfig) (net.Conn, error) {
				return tlsDialer.DialTLSContext(ctx, M.ParseSocksaddr(addr))
			},
		}
	}

	var host string
	if options.Host != "" {
		host = options.Host
	} else if tlsConfig != nil && tlsConfig.ServerName() != "" {
		host = tlsConfig.ServerName()
	} else {
		host = serverAddr.AddrString()
	}

	var requestURL url.URL
	if tlsConfig == nil {
		requestURL.Scheme = "http"
	} else {
		requestURL.Scheme = "https"
	}
	requestURL.Host = serverAddr.String()

	path := options.Path
	if path == "" {
		path = "/"
	}
	err := sHTTP.URLSetPath(&requestURL, path)
	if err != nil {
		return nil, E.Cause(err, "parse path")
	}
	if !strings.HasPrefix(requestURL.Path, "/") {
		requestURL.Path = "/" + requestURL.Path
	}
	if !strings.HasSuffix(requestURL.Path, "/") {
		requestURL.Path = requestURL.Path + "/"
	}

	mode := options.Mode
	if mode == "" {
		mode = C.XHTTPModeAuto
	}

	return &Client{
		ctx:                  ctx,
		dialer:               dialer,
		serverAddr:           serverAddr,
		requestURL:           requestURL,
		host:                 host,
		headers:              options.Headers.Build(),
		mode:                 mode,
		noGRPCHeader:         options.NoGRPCHeader,
		noSSEHeader:          options.NoSSEHeader,
		xPaddingBytes:        options.XPaddingBytes,
		scMaxEachPostBytes:   options.ScMaxEachPostBytes,
		scMinPostsIntervalMs: options.ScMinPostsIntervalMs,
		transport:            transport,
		http2:                tlsConfig != nil,
	}, nil
}

func (c *Client) DialContext(ctx context.Context) (net.Conn, error) {
	mode := c.mode
	if mode == C.XHTTPModeAuto {
		mode = C.XHTTPModePacketUp
	}

	switch mode {
	case C.XHTTPModeStreamOne:
		return c.dialStreamOne(ctx)
	case C.XHTTPModeStreamUp:
		return c.dialStreamUp(ctx)
	case C.XHTTPModePacketUp:
		return c.dialPacketUp(ctx)
	default:
		return c.dialPacketUp(ctx)
	}
}

func (c *Client) dialStreamOne(ctx context.Context) (net.Conn, error) {
	sessionID := c.generateSessionID()
	requestURL := c.requestURL

	pipeReader, pipeWriter := io.Pipe()
	request := &http.Request{
		Method: http.MethodPost,
		URL:    &requestURL,
		Header: c.headers.Clone(),
		Body:   pipeReader,
		Host:   c.host,
	}
	request = request.WithContext(ctx)

	c.setRequestHeaders(request, true)

	waitReader := newWaitReadCloser()

	go func() {
		response, err := c.transport.RoundTrip(request)
		if err != nil {
			waitReader.Set(nil, err)
			return
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			waitReader.Set(nil, E.New("xhttp: unexpected status: ", response.Status))
			return
		}
		waitReader.Set(response.Body, nil)
	}()

	return newSplitConn(
		waitReader,
		&pipeWriteCloser{pipeWriter},
		dummyAddr{network: "tcp", address: c.serverAddr.String()},
		dummyAddr{network: "tcp", address: sessionID},
		nil,
	), nil
}

func (c *Client) dialStreamUp(ctx context.Context) (net.Conn, error) {
	sessionID := c.generateSessionID()
	requestURL := c.requestURL
	requestURL.Path = requestURL.Path + sessionID

	// Start download connection (GET)
	downloadReader := c.startDownload(ctx, requestURL)

	// Start upload connection (POST with streaming body)
	pipeReader, pipeWriter := io.Pipe()
	uploadRequest := &http.Request{
		Method: http.MethodPost,
		URL:    &requestURL,
		Header: c.headers.Clone(),
		Body:   pipeReader,
		Host:   c.host,
	}
	uploadRequest = uploadRequest.WithContext(ctx)
	c.setRequestHeaders(uploadRequest, true)

	go func() {
		response, err := c.transport.RoundTrip(uploadRequest)
		if err != nil {
			return
		}
		io.Copy(io.Discard, response.Body)
		response.Body.Close()
	}()

	return newSplitConn(
		downloadReader,
		&pipeWriteCloser{pipeWriter},
		dummyAddr{network: "tcp", address: c.serverAddr.String()},
		dummyAddr{network: "tcp", address: sessionID},
		nil,
	), nil
}

func (c *Client) dialPacketUp(ctx context.Context) (net.Conn, error) {
	sessionID := c.generateSessionID()
	baseURL := c.requestURL
	baseURL.Path = baseURL.Path + sessionID

	// Start download connection (GET)
	downloadReader := c.startDownload(ctx, baseURL)

	// Create upload writer using packet mode
	uploadWriter := newPacketUpWriter(ctx, c, baseURL, sessionID)

	return newSplitConn(
		downloadReader,
		uploadWriter,
		dummyAddr{network: "tcp", address: c.serverAddr.String()},
		dummyAddr{network: "tcp", address: sessionID},
		nil,
	), nil
}

func (c *Client) startDownload(ctx context.Context, baseURL url.URL) *waitReadCloser {
	downloadURL := baseURL
	request := &http.Request{
		Method: http.MethodGet,
		URL:    &downloadURL,
		Header: c.headers.Clone(),
		Host:   c.host,
	}
	request = request.WithContext(ctx)
	c.setRequestHeaders(request, false)

	waitReader := newWaitReadCloser()

	go func() {
		response, err := c.transport.RoundTrip(request)
		if err != nil {
			waitReader.Set(nil, err)
			return
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			waitReader.Set(nil, E.New("xhttp: download unexpected status: ", response.Status))
			return
		}
		waitReader.Set(response.Body, nil)
	}()

	return waitReader
}

func (c *Client) setRequestHeaders(request *http.Request, isUpload bool) {
	padding := c.generatePadding()
	if request.URL.RawQuery != "" {
		request.URL.RawQuery += "&x_padding=" + padding
	} else {
		request.URL.RawQuery = "x_padding=" + padding
	}

	if isUpload && !c.noGRPCHeader {
		request.Header.Set("Content-Type", "application/grpc")
	}
}

func (c *Client) generateSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (c *Client) generatePadding() string {
	minBytes := int32(100)
	maxBytes := int32(1000)
	if c.xPaddingBytes != nil {
		minBytes = c.xPaddingBytes.GetFrom(100)
		maxBytes = c.xPaddingBytes.GetTo(1000)
	}
	length := minBytes
	if maxBytes > minBytes {
		b := make([]byte, 1)
		rand.Read(b)
		length = minBytes + int32(b[0])%(maxBytes-minBytes+1)
	}
	padding := make([]byte, length)
	for i := range padding {
		padding[i] = 'A' + byte(i%26)
	}
	return string(padding)
}

func (c *Client) Close() error {
	if t, ok := c.transport.(*http.Transport); ok {
		t.CloseIdleConnections()
	}
	if t, ok := c.transport.(*http2.Transport); ok {
		t.CloseIdleConnections()
	}
	return nil
}

// packetUpWriter implements io.WriteCloser for packet-up mode
type packetUpWriter struct {
	ctx       context.Context
	client    *Client
	baseURL   url.URL
	sessionID string
	seq       int
	mu        sync.Mutex
	closed    bool
}

func newPacketUpWriter(ctx context.Context, client *Client, baseURL url.URL, sessionID string) *packetUpWriter {
	return &packetUpWriter{
		ctx:       ctx,
		client:    client,
		baseURL:   baseURL,
		sessionID: sessionID,
		seq:       0,
	}
}

func (w *packetUpWriter) Write(b []byte) (n int, err error) {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return 0, io.ErrClosedPipe
	}
	seq := w.seq
	w.seq++
	w.mu.Unlock()

	uploadURL := w.baseURL
	uploadURL.Path = fmt.Sprintf("%s/%d", uploadURL.Path, seq)

	request := &http.Request{
		Method:        http.MethodPost,
		URL:           &uploadURL,
		Header:        w.client.headers.Clone(),
		Body:          io.NopCloser(strings.NewReader(string(b))),
		ContentLength: int64(len(b)),
		Host:          w.client.host,
	}
	request = request.WithContext(w.ctx)
	w.client.setRequestHeaders(request, true)

	response, err := w.client.transport.RoundTrip(request)
	if err != nil {
		return 0, err
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return 0, E.New("xhttp: upload unexpected status: ", response.Status)
	}

	return len(b), nil
}

func (w *packetUpWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	return nil
}
