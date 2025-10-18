package xhttp

import (
	"context"
	gotls "crypto/tls"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/common/xray/buf"
	"github.com/sagernet/sing-box/common/xray/net"
	"github.com/sagernet/sing-box/common/xray/pipe"
	"github.com/sagernet/sing-box/common/xray/signal/done"
	"github.com/sagernet/sing-box/common/xray/uuid"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	sHTTP "github.com/sagernet/sing/protocol/http"
	"github.com/sagernet/sing/service"
	"golang.org/x/net/http2"
)

type Client struct {
	ctx            context.Context
	options        *option.V2RayXHTTPOptions
	getRequestURL  func(sessionId string) url.URL
	getRequestURL2 func(sessionId string) url.URL
	getHTTPClient  func() (DialerClient, *XmuxClient)
	getHTTPClient2 func() (DialerClient, *XmuxClient)
}

func NewClient(ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr, options option.V2RayXHTTPOptions, tlsConfig tls.Config) (adapter.V2RayClientTransport, error) {
	if options.Mode == "" {
		return nil, E.New("mode is not set")
	}
	dest := serverAddr
	var gotlsConfig *gotls.Config
	if tlsConfig != nil {
		var err error
		gotlsConfig, err = tlsConfig.Config()
		if err != nil {
			return nil, err
		}
	}
	baseRequestURL, err := getBaseRequestURL(
		&options.V2RayXHTTPBaseOptions, dest, tlsConfig,
	)
	if err != nil {
		return nil, err
	}
	getRequestURL := func(sessionId string) url.URL {
		requestURL := baseRequestURL
		requestURL.Path += sessionId
		return requestURL
	}
	var xmuxOptions option.V2RayXHTTPXmuxOptions
	if options.Xmux != nil {
		xmuxOptions = *options.Xmux
	}
	xmuxManager := NewXmuxManager(xmuxOptions, func() XmuxConn {
		return createHTTPClient(dest, dialer, &options.V2RayXHTTPBaseOptions, tlsConfig, gotlsConfig)
	})
	getHTTPClient := func() (DialerClient, *XmuxClient) {
		xmuxClient := xmuxManager.GetXmuxClient(ctx)
		return xmuxClient.XmuxConn.(DialerClient), xmuxClient
	}
	getRequestURL2 := getRequestURL
	getHTTPClient2 := getHTTPClient
	if options.Download != nil {
		options2 := options.Download
		dialer2 := dialer
		if options2.Detour != "" {
			var ok bool
			dialer2, ok = service.FromContext[adapter.OutboundManager](ctx).Outbound(options2.Detour)
			if !ok {
				return nil, E.New("outbound detour not found: ", options2.Detour)
			}
		}
		dest2 := options2.ServerOptions.Build()
		var tlsConfig2 tls.Config
		var gotlsConfig2 *gotls.Config
		if options2.TLS != nil {
			tlsConfig2, err = tls.NewClient(ctx, options2.Server, common.PtrValueOrDefault(options2.TLS))
			if err != nil {
				return nil, err
			}
			gotlsConfig2, err = tlsConfig2.Config()
			if err != nil {
				return nil, err
			}
		}
		baseRequestURL2, err := getBaseRequestURL(&options2.V2RayXHTTPBaseOptions, dest2, tlsConfig2)
		if err != nil {
			return nil, err
		}
		getRequestURL2 = func(sessionId string) url.URL {
			requestURL2 := baseRequestURL2
			requestURL2.Path += sessionId
			return requestURL2
		}
		var xmuxOptions2 option.V2RayXHTTPXmuxOptions
		if options2.Xmux != nil {
			xmuxOptions2 = *options2.Xmux
		}
		xmuxManager2 := NewXmuxManager(xmuxOptions2, func() XmuxConn {
			return createHTTPClient(dest2, dialer2, &options2.V2RayXHTTPBaseOptions, tlsConfig2, gotlsConfig2)
		})
		getHTTPClient2 = func() (DialerClient, *XmuxClient) {
			xmuxClient2 := xmuxManager2.GetXmuxClient(ctx)
			return xmuxClient2.XmuxConn.(DialerClient), xmuxClient2
		}
	}
	return &Client{
		ctx:            ctx,
		options:        &options,
		getHTTPClient:  getHTTPClient,
		getHTTPClient2: getHTTPClient2,
		getRequestURL:  getRequestURL,
		getRequestURL2: getRequestURL2,
	}, nil
}

func (c *Client) DialContext(ctx context.Context) (net.Conn, error) {
	options := c.options
	mode := c.options.Mode
	sessionIdUuid := uuid.New()
	requestURL := c.getRequestURL(sessionIdUuid.String())
	requestURL2 := c.getRequestURL(sessionIdUuid.String())
	httpClient, xmuxClient := c.getHTTPClient()
	httpClient2, xmuxClient2 := c.getHTTPClient2()
	if xmuxClient != nil {
		xmuxClient.OpenUsage.Add(1)
	}
	if xmuxClient2 != nil && xmuxClient2 != xmuxClient {
		xmuxClient2.OpenUsage.Add(1)
	}
	var closed atomic.Int32
	reader, writer := io.Pipe()
	conn := splitConn{
		writer: writer,
		onClose: func() {
			if closed.Add(1) > 1 {
				return
			}
			if xmuxClient != nil {
				xmuxClient.OpenUsage.Add(-1)
			}
			if xmuxClient2 != nil && xmuxClient2 != xmuxClient {
				xmuxClient2.OpenUsage.Add(-1)
			}
		},
	}
	var err error
	if mode == "stream-one" {
		requestURL.Path = options.GetNormalizedPath()
		if xmuxClient != nil {
			xmuxClient.LeftRequests.Add(-1)
		}
		conn.reader, conn.remoteAddr, conn.localAddr, err = httpClient.OpenStream(ctx, requestURL.String(), reader, false)
		if err != nil { // browser dialer only
			return nil, err
		}
		return &conn, nil
	} else { // stream-down
		if xmuxClient2 != nil {
			xmuxClient2.LeftRequests.Add(-1)
		}
		conn.reader, conn.remoteAddr, conn.localAddr, err = httpClient2.OpenStream(ctx, requestURL2.String(), nil, false)
		if err != nil { // browser dialer only
			return nil, err
		}
	}
	if mode == "stream-up" {
		if xmuxClient != nil {
			xmuxClient.LeftRequests.Add(-1)
		}
		_, _, _, err = httpClient.OpenStream(ctx, requestURL.String(), reader, true)
		if err != nil { // browser dialer only
			return nil, err
		}
		return &conn, nil
	}
	scMaxEachPostBytes := options.GetNormalizedScMaxEachPostBytes()
	scMinPostsIntervalMs := options.GetNormalizedScMinPostsIntervalMs()
	if scMaxEachPostBytes.From <= buf.Size {
		panic("`scMaxEachPostBytes` should be bigger than " + strconv.Itoa(buf.Size))
	}
	maxUploadSize := scMaxEachPostBytes.Rand()
	// WithSizeLimit(0) will still allow single bytes to pass, and a lot of
	// code relies on this behavior. Subtract 1 so that together with
	// uploadWriter wrapper, exact size limits can be enforced
	// uploadPipeReader, uploadPipeWriter := pipe.New(pipe.WithSizeLimit(maxUploadSize - 1))
	uploadPipeReader, uploadPipeWriter := pipe.New(pipe.WithSizeLimit(maxUploadSize - buf.Size))
	conn.writer = uploadWriter{
		uploadPipeWriter,
		maxUploadSize,
	}
	go func() {
		var seq int64
		var lastWrite time.Time
		for {
			wroteRequest := done.New()
			ctx := httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
				WroteRequest: func(httptrace.WroteRequestInfo) {
					wroteRequest.Close()
				},
			})
			// this intentionally makes a shallow-copy of the struct so we
			// can reassign Path (potentially concurrently)
			url := requestURL
			url.Path += "/" + strconv.FormatInt(seq, 10)
			seq += 1
			if scMinPostsIntervalMs.From > 0 {
				time.Sleep(time.Duration(scMinPostsIntervalMs.Rand())*time.Millisecond - time.Since(lastWrite))
			}
			// by offloading the uploads into a buffered pipe, multiple conn.Write
			// calls get automatically batched together into larger POST requests.
			// without batching, bandwidth is extremely limited.
			chunk, err := uploadPipeReader.ReadMultiBuffer()
			if err != nil {
				break
			}
			lastWrite = time.Now()
			if xmuxClient != nil && (xmuxClient.LeftRequests.Add(-1) <= 0 ||
				(xmuxClient.UnreusableAt != time.Time{} && lastWrite.After(xmuxClient.UnreusableAt))) {
				httpClient, xmuxClient = c.getHTTPClient()
			}
			go func() {
				err := httpClient.PostPacket(
					ctx,
					url.String(),
					&buf.MultiBufferContainer{MultiBuffer: chunk},
					int64(chunk.Len()),
				)
				wroteRequest.Close()
				if err != nil {
					uploadPipeReader.Interrupt()
				}
			}()
			if _, ok := httpClient.(*DefaultDialerClient); ok {
				<-wroteRequest.Wait()
			}
		}
	}()
	return &conn, nil
}

func (c *Client) Close() error {
	return nil
}

func decideHTTPVersion(gotlsConfig *gotls.Config) string {
	if gotlsConfig == nil || len(gotlsConfig.NextProtos) == 0 || gotlsConfig.NextProtos[0] == "http/1.1" {
		return "1.1"
	}
	if gotlsConfig.NextProtos[0] == "h3" {
		return "3"
	}
	return "2"
}

func getBaseRequestURL(options *option.V2RayXHTTPBaseOptions, dest M.Socksaddr, tlsConfig tls.Config) (url.URL, error) {
	var requestURL url.URL
	if tlsConfig == nil {
		requestURL.Scheme = "http"
	} else {
		requestURL.Scheme = "https"
	}
	requestURL.Host = options.Host
	if requestURL.Host == "" && tlsConfig != nil {
		requestURL.Host = tlsConfig.ServerName()
	}
	if requestURL.Host == "" {
		requestURL.Host = dest.AddrString()
	}
	requestURL.Path = options.Path
	if err := sHTTP.URLSetPath(&requestURL, options.Path); err != nil {
		return requestURL, E.New(err, "parse path")
	}
	if !strings.HasPrefix(requestURL.Path, "/") {
		requestURL.Path = "/" + requestURL.Path
	}
	requestURL.Path = options.GetNormalizedPath()
	requestURL.RawQuery = options.GetNormalizedQuery()
	return requestURL, nil
}

func createHTTPClient(dest M.Socksaddr, dialer N.Dialer, options *option.V2RayXHTTPBaseOptions, tlsConfig tls.Config, gotlsConfig *gotls.Config) DialerClient {
	httpVersion := decideHTTPVersion(gotlsConfig)
	dialContext := func(ctxInner context.Context) (net.Conn, error) {
		conn, err := dialer.DialContext(ctxInner, "tcp", dest)
		if err != nil {
			return nil, err
		}
		if httpVersion == "2" {
			return tls.ClientHandshake(ctxInner, conn, tlsConfig)
		}
		return conn, nil
	}
	var keepAlivePeriod time.Duration
	if options.Xmux != nil {
		keepAlivePeriod = time.Duration(options.Xmux.HKeepAlivePeriod) * time.Second
	}
	var transport http.RoundTripper
	switch httpVersion {
	case "3":
		if keepAlivePeriod == 0 {
			keepAlivePeriod = net.QuicgoH3KeepAlivePeriod
		}
		if keepAlivePeriod < 0 {
			keepAlivePeriod = 0
		}
		quicConfig := &quic.Config{
			MaxIdleTimeout: net.ConnIdleTimeout,
			// these two are defaults of quic-go/http3. the default of quic-go (no
			// http3) is different, so it is hardcoded here for clarity.
			// https://github.com/quic-go/quic-go/blob/b8ea5c798155950fb5bbfdd06cad1939c9355878/http3/client.go#L36-L39
			MaxIncomingStreams: -1,
			KeepAlivePeriod:    keepAlivePeriod,
		}
		transport = &http3.Transport{
			QUICConfig:      quicConfig,
			TLSClientConfig: gotlsConfig.Clone(),
			Dial: func(ctx context.Context, addr string, tlsCfg *gotls.Config, cfg *quic.Config) (*quic.Conn, error) {
				udpConn, dErr := dialer.DialContext(ctx, N.NetworkUDP, dest)
				if dErr != nil {
					return nil, dErr
				}
				return quic.DialEarly(ctx, bufio.NewUnbindPacketConn(udpConn), udpConn.RemoteAddr(), tlsCfg, cfg)
			},
		}
	case "2":
		if keepAlivePeriod == 0 {
			keepAlivePeriod = net.ChromeH2KeepAlivePeriod
		}
		if keepAlivePeriod < 0 {
			keepAlivePeriod = 0
		}
		transport = &http2.Transport{
			DialTLSContext: func(ctxInner context.Context, network string, addr string, cfg *gotls.Config) (net.Conn, error) {
				return dialContext(ctxInner)
			},
			IdleConnTimeout: net.ConnIdleTimeout,
			ReadIdleTimeout: keepAlivePeriod,
		}
	default:
		httpDialContext := func(ctxInner context.Context, network string, addr string) (net.Conn, error) {
			return dialContext(ctxInner)
		}
		transport = &http.Transport{
			DialTLSContext:  httpDialContext,
			DialContext:     httpDialContext,
			IdleConnTimeout: net.ConnIdleTimeout,
			// chunked transfer download with KeepAlives is buggy with
			// http.Client and our custom dial context.
			DisableKeepAlives: true,
		}
	}
	client := &DefaultDialerClient{
		options: options,
		client: &http.Client{
			Transport: transport,
		},
		httpVersion:    httpVersion,
		uploadRawPool:  &sync.Pool{},
		dialUploadConn: dialContext,
	}
	return client
}
