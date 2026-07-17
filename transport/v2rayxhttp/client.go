package v2rayxhttp

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"os"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"golang.org/x/net/http2"
)

var _ adapter.V2RayClientTransport = (*Client)(nil)

type Client struct {
	ctx      context.Context
	upload   *clientTarget
	download *clientTarget
}

type clientTarget struct {
	serverAddr M.Socksaddr
	config     *config
	xmux       *xmuxManager
	requestURL string
	reality    bool
}

func NewClient(ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr, options option.V2RayXHTTPOptions, tlsConfig tls.Config) (*Client, error) {
	config, err := newConfig(options)
	if err != nil {
		return nil, err
	}
	if options.DownloadSettings != nil && config.mode == "stream-one" {
		return nil, E.New("xhttp download_settings cannot be used with stream-one")
	}
	upload, err := newClientTarget(dialer, serverAddr, config, tlsConfig)
	if err != nil {
		return nil, err
	}
	client := &Client{ctx: ctx, upload: upload}
	if options.DownloadSettings == nil {
		return client, nil
	}
	downloadOptions := options.DownloadSettings
	if downloadOptions.DownloadSettings != nil {
		return nil, E.New("nested xhttp download_settings is unsupported")
	}
	downloadAddr := downloadOptions.ServerOptions.Build()
	if !downloadAddr.IsValid() {
		return nil, E.New("invalid xhttp download_settings server")
	}
	downloadConfig, err := newConfig(downloadOptions.V2RayXHTTPOptions)
	if err != nil {
		return nil, E.Cause(err, "invalid xhttp download_settings")
	}
	downloadTLS, err := tls.NewClient(ctx, logger.NOP(), downloadOptions.Server, common.PtrValueOrDefault(downloadOptions.TLS))
	if err != nil {
		return nil, E.Cause(err, "create xhttp download_settings tls client")
	}
	download, err := newClientTarget(dialer, downloadAddr, downloadConfig, downloadTLS)
	if err != nil {
		return nil, err
	}
	client.download = download
	return client, nil
}

func newClientTarget(dialer N.Dialer, serverAddr M.Socksaddr, config *config, tlsConfig tls.Config) (*clientTarget, error) {
	scheme := "http"
	var newTransport func() http.RoundTripper
	var err error
	if isHTTP3(tlsConfig) {
		scheme = "https"
		newTransport, err = newHTTP3TransportFactory(dialer, serverAddr, tlsConfig, config.quic)
		if err != nil {
			return nil, err
		}
	} else if tlsConfig == nil {
		newTransport = func() http.RoundTripper {
			return &http.Transport{
				DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
					return dialer.DialContext(ctx, network, serverAddr)
				},
				MaxIdleConns: 64, MaxIdleConnsPerHost: 64, IdleConnTimeout: 30 * time.Minute,
			}
		}
	} else {
		scheme = "https"
		if len(tlsConfig.NextProtos()) == 0 {
			tlsConfig.SetNextProtos([]string{http2.NextProtoTLS})
		}
		tlsDialer := tls.NewDialer(dialer, tlsConfig)
		if nextProtos := tlsConfig.NextProtos(); len(nextProtos) == 1 && nextProtos[0] == "http/1.1" {
			newTransport = func() http.RoundTripper {
				return &http.Transport{
					DialTLSContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
						return tlsDialer.DialTLSContext(ctx, serverAddr)
					},
					DisableKeepAlives: true,
					MaxIdleConns:      64, MaxIdleConnsPerHost: 64, IdleConnTimeout: 30 * time.Minute,
				}
			}
		} else {
			readIdleTimeout := 30 * time.Second
			if config.xmux.keepAlivePeriod > 0 {
				readIdleTimeout = config.xmux.keepAlivePeriod
			}
			newTransport = func() http.RoundTripper {
				return &http2.Transport{
					DialTLSContext: func(ctx context.Context, network, _ string, _ *tls.STDConfig) (net.Conn, error) {
						return tlsDialer.DialTLSContext(ctx, serverAddr)
					},
					ReadIdleTimeout: readIdleTimeout,
					PingTimeout:     15 * time.Second,
				}
			}
		}
	}
	host := serverAddr.String()
	if config.host != "" {
		host = config.host
	}
	requestURLValue := config.requestURL(scheme, host)
	requestURL := requestURLValue.String()
	return &clientTarget{serverAddr: serverAddr, config: config, xmux: newXMuxManager(config.xmux, newTransport), requestURL: requestURL, reality: isRealityClient(tlsConfig)}, nil
}

func (c *Client) DialContext(ctx context.Context) (net.Conn, error) {
	ctx, cancel := context.WithCancel(ctx)
	uploadClient := c.upload.xmux.acquireConnection()
	downloadTarget, downloadClient := c.upload, uploadClient
	if c.download != nil {
		downloadTarget = c.download
		downloadClient = c.download.xmux.acquireConnection()
	}
	closeXMuxClients := func() {
		c.upload.xmux.doneConnection(uploadClient)
		if c.download != nil {
			c.download.xmux.doneConnection(downloadClient)
		}
	}
	mode := resolvedMode(c.upload.config.mode, c.upload.reality, c.download != nil)
	sessionID := ""
	if mode != "stream-one" {
		sessionID = c.upload.config.newSessionID()
	}
	if mode == "stream-one" {
		reader, writer := io.Pipe()
		response, err := c.upload.openStream(ctx, uploadClient.transport, http.MethodPost, sessionID, reader)
		if err != nil {
			cancel()
			_ = writer.Close()
			closeXMuxClients()
			return nil, err
		}
		return &splitConn{reader: response.Body, writer: writer, cancel: cancel, onClose: closeXMuxClients}, nil
	}
	response, err := downloadTarget.openStream(ctx, downloadClient.transport, http.MethodGet, sessionID, nil)
	if err != nil {
		cancel()
		closeXMuxClients()
		return nil, err
	}
	if mode == "stream-up" {
		reader, writer := io.Pipe()
		go func() {
			// Xray keeps the paired stream-up request on the same selected
			// client even if this consumes its final request budget.
			c.upload.xmux.consumeRequest(uploadClient)
			response, err := c.upload.openStream(ctx, uploadClient.transport, c.upload.config.uplinkMethod, sessionID, reader)
			if err != nil {
				_ = reader.CloseWithError(err)
				return
			}
			_, err = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			if err != nil {
				_ = reader.CloseWithError(err)
			}
		}()
		return &splitConn{reader: response.Body, writer: writer, cancel: cancel, onClose: closeXMuxClients}, nil
	}
	writer := newPacketWriter(ctx, c.upload, uploadClient, sessionID)
	return &splitConn{reader: response.Body, writer: writer, cancel: cancel, onClose: closeXMuxClients}, nil
}

func resolvedMode(mode string, reality, downloadSettings bool) string {
	if mode != "auto" {
		return mode
	}
	if reality {
		if downloadSettings {
			return "stream-up"
		}
		return "stream-one"
	}
	return "packet-up"
}

func (c *clientTarget) openStream(ctx context.Context, transport http.RoundTripper, method, sessionID string, body io.Reader) (*http.Response, error) {
	// The logical connection can outlive the routing context that initiated
	// DialContext. Xray deliberately detaches its streaming HTTP request here;
	// the pipe/response bodies still provide the transport close signal.
	request, err := http.NewRequestWithContext(context.WithoutCancel(ctx), method, c.requestURL, body)
	if err != nil {
		return nil, err
	}
	c.config.fillStreamRequest(request, sessionID)
	reader := newWaitingResponseBody()
	gotConnection := make(chan struct{})
	var gotConnectionOnce sync.Once
	notifyConnection := func() { gotConnectionOnce.Do(func() { close(gotConnection) }) }
	request = request.WithContext(httptrace.WithClientTrace(request.Context(), &httptrace.ClientTrace{
		GotConn: func(httptrace.GotConnInfo) { notifyConnection() },
	}))
	go func() {
		response, roundTripErr := transport.RoundTrip(request)
		if roundTripErr != nil {
			reader.set(nil, roundTripErr)
			notifyConnection()
			return
		}
		if response.StatusCode != http.StatusOK {
			_ = response.Body.Close()
			reader.set(nil, E.New("xhttp: unexpected status: ", response.Status))
			notifyConnection()
			return
		}
		reader.set(response.Body, nil)
		notifyConnection()
	}()
	select {
	case <-gotConnection:
		return &http.Response{StatusCode: http.StatusOK, Body: reader}, nil
	case <-ctx.Done():
		_ = reader.Close()
		return nil, ctx.Err()
	}
}

type waitingResponseBody struct {
	ready  chan struct{}
	access sync.Mutex
	body   io.ReadCloser
	err    error
	closed bool
}

func newWaitingResponseBody() *waitingResponseBody {
	return &waitingResponseBody{ready: make(chan struct{})}
}

func (r *waitingResponseBody) set(body io.ReadCloser, err error) {
	r.access.Lock()
	if r.closed && body != nil {
		_ = body.Close()
	}
	r.body, r.err = body, err
	close(r.ready)
	r.access.Unlock()
}

func (r *waitingResponseBody) Read(buffer []byte) (int, error) {
	<-r.ready
	r.access.Lock()
	body, err := r.body, r.err
	r.access.Unlock()
	if err != nil {
		return 0, err
	}
	if body == nil {
		return 0, io.ErrClosedPipe
	}
	return body.Read(buffer)
}

func (r *waitingResponseBody) Close() error {
	r.access.Lock()
	r.closed = true
	body := r.body
	r.access.Unlock()
	if body != nil {
		return body.Close()
	}
	return nil
}

func (c *clientTarget) sendPacket(ctx context.Context, transport http.RoundTripper, sessionID string, sequence uint64, payload []byte) error {
	request, err := http.NewRequestWithContext(ctx, c.config.uplinkMethod, c.requestURL, nil)
	if err != nil {
		return err
	}
	c.config.fillPacketRequest(request, sessionID, sequence, payload)
	response, err := transport.RoundTrip(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	if response.StatusCode != http.StatusOK {
		return E.New("xhttp: unexpected upload status: ", response.Status)
	}
	return nil
}

func (c *Client) Close() error {
	c.upload.xmux.Close()
	if c.download != nil {
		c.download.xmux.Close()
	}
	return nil
}

type splitConn struct {
	reader  io.ReadCloser
	writer  io.WriteCloser
	cancel  context.CancelFunc
	onClose func()
	once    sync.Once
}

func (c *splitConn) Read(buffer []byte) (int, error)  { return c.reader.Read(buffer) }
func (c *splitConn) Write(buffer []byte) (int, error) { return c.writer.Write(buffer) }
func (c *splitConn) Close() error {
	var err error
	c.once.Do(func() {
		c.cancel()
		err = common.Close(c.writer, c.reader)
		if c.onClose != nil {
			c.onClose()
		}
	})
	return err
}
func (c *splitConn) LocalAddr() net.Addr              { return M.Socksaddr{} }
func (c *splitConn) RemoteAddr() net.Addr             { return M.Socksaddr{} }
func (c *splitConn) SetDeadline(time.Time) error      { return os.ErrInvalid }
func (c *splitConn) SetReadDeadline(time.Time) error  { return os.ErrInvalid }
func (c *splitConn) SetWriteDeadline(time.Time) error { return os.ErrInvalid }

type packetWriter struct {
	ctx        context.Context
	target     *clientTarget
	xmuxClient *xmuxClient
	sessionID  string
	packets    chan []byte
	closed     chan struct{}
	once       sync.Once
	access     sync.Mutex
	err        error
}

func newPacketWriter(ctx context.Context, target *clientTarget, xmuxClient *xmuxClient, sessionID string) *packetWriter {
	writer := &packetWriter{ctx: ctx, target: target, xmuxClient: xmuxClient, sessionID: sessionID, packets: make(chan []byte, target.config.maxBufferedPosts), closed: make(chan struct{})}
	go writer.run()
	return writer
}
func (w *packetWriter) Write(payload []byte) (int, error) {
	w.access.Lock()
	err := w.err
	w.access.Unlock()
	if err != nil {
		return 0, err
	}
	copyPayload := bytes.Clone(payload)
	select {
	case w.packets <- copyPayload:
		return len(payload), nil
	case <-w.closed:
		return 0, net.ErrClosed
	case <-w.ctx.Done():
		return 0, w.ctx.Err()
	}
}
func (w *packetWriter) Close() error { w.once.Do(func() { close(w.closed) }); return nil }
func (w *packetWriter) run() {
	var sequence uint64
	var remainder []byte
	for {
		select {
		case first := <-w.packets:
			limit := w.target.config.scMaxPost.random()
			payload := append([]byte(nil), remainder...)
			remainder = nil
			payload = append(payload, first...)
			for len(payload) < limit {
				select {
				case next := <-w.packets:
					payload = append(payload, next...)
				default:
					goto flush
				}
			}
		flush:
			if len(payload) > limit {
				remainder = append(remainder, payload[limit:]...)
				payload = payload[:limit]
			}
			if !w.target.xmux.consumePacketRequest(w.xmuxClient) {
				w.xmuxClient = w.target.xmux.acquireRequest()
			}
			if err := w.target.sendPacket(w.ctx, w.xmuxClient.transport, w.sessionID, sequence, payload); err != nil {
				w.access.Lock()
				w.err = err
				w.access.Unlock()
				return
			}
			sequence++
			interval := time.Duration(w.target.config.scMinInterval.random()) * time.Millisecond
			if interval > 0 {
				timer := time.NewTimer(interval)
				select {
				case <-timer.C:
				case <-w.closed:
					timer.Stop()
					return
				case <-w.ctx.Done():
					timer.Stop()
					return
				}
			}
		case <-w.closed:
			return
		case <-w.ctx.Done():
			return
		}
	}
}
