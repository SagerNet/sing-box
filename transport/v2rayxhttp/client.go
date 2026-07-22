package v2rayxhttp

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"golang.org/x/net/http2"
)

var _ adapter.V2RayClientTransport = (*Client)(nil)

type Client struct {
	ctx        context.Context
	dialer     N.Dialer
	serverAddr M.Socksaddr
	config     *config
	transport  http.RoundTripper
	requestURL string
}

func NewClient(ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr, options option.V2RayXHTTPOptions, tlsConfig tls.Config) (*Client, error) {
	config, err := newConfig(options)
	if err != nil {
		return nil, err
	}
	scheme := "http"
	var transport http.RoundTripper
	if tlsConfig == nil {
		transport = &http.Transport{
			DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, serverAddr)
			},
			MaxIdleConns: 64, MaxIdleConnsPerHost: 64, IdleConnTimeout: 30 * time.Minute,
		}
	} else {
		scheme = "https"
		if len(tlsConfig.NextProtos()) == 0 {
			tlsConfig.SetNextProtos([]string{http2.NextProtoTLS})
		}
		tlsDialer := tls.NewDialer(dialer, tlsConfig)
		if nextProtos := tlsConfig.NextProtos(); len(nextProtos) == 1 && nextProtos[0] == "http/1.1" {
			transport = &http.Transport{
				DialTLSContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
					return tlsDialer.DialTLSContext(ctx, serverAddr)
				},
				MaxIdleConns: 64, MaxIdleConnsPerHost: 64, IdleConnTimeout: 30 * time.Minute,
			}
		} else {
			transport = &http2.Transport{
				DialTLSContext: func(ctx context.Context, network, _ string, _ *tls.STDConfig) (net.Conn, error) {
					return tlsDialer.DialTLSContext(ctx, serverAddr)
				},
				ReadIdleTimeout: 30 * time.Second,
				PingTimeout:     15 * time.Second,
			}
		}
	}
	host := serverAddr.String()
	if config.host != "" {
		host = config.host
	}
	requestURLValue := config.requestURL(scheme, host)
	requestURL := requestURLValue.String()
	return &Client{ctx: ctx, dialer: dialer, serverAddr: serverAddr, config: config, transport: transport, requestURL: requestURL}, nil
}

func (c *Client) DialContext(ctx context.Context) (net.Conn, error) {
	ctx, cancel := context.WithCancel(ctx)
	mode := c.config.mode
	if mode == "auto" {
		mode = "packet-up"
	}
	sessionID := ""
	if mode != "stream-one" {
		sessionID = c.config.newSessionID()
	}
	if mode == "stream-one" {
		reader, writer := io.Pipe()
		response, err := c.openStream(ctx, http.MethodPost, sessionID, reader)
		if err != nil {
			cancel()
			_ = writer.Close()
			return nil, err
		}
		return &splitConn{reader: response.Body, writer: writer, cancel: cancel}, nil
	}
	response, err := c.openStream(ctx, http.MethodGet, sessionID, nil)
	if err != nil {
		cancel()
		return nil, err
	}
	if mode == "stream-up" {
		reader, writer := io.Pipe()
		go func() {
			_, err := c.openStream(ctx, c.config.uplinkMethod, sessionID, reader)
			if err != nil {
				_ = reader.CloseWithError(err)
			}
		}()
		return &splitConn{reader: response.Body, writer: writer, cancel: cancel}, nil
	}
	writer := newPacketWriter(ctx, c, sessionID)
	return &splitConn{reader: response.Body, writer: writer, cancel: cancel}, nil
}

func (c *Client) openStream(ctx context.Context, method, sessionID string, body io.Reader) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, method, c.requestURL, body)
	if err != nil {
		return nil, err
	}
	c.config.fillStreamRequest(request, sessionID)
	response, err := c.transport.RoundTrip(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		return nil, E.New("xhttp: unexpected status: ", response.Status)
	}
	return response, nil
}

func (c *Client) sendPacket(ctx context.Context, sessionID string, sequence uint64, payload []byte) error {
	request, err := http.NewRequestWithContext(ctx, c.config.uplinkMethod, c.requestURL, nil)
	if err != nil {
		return err
	}
	c.config.fillPacketRequest(request, sessionID, sequence, payload)
	response, err := c.transport.RoundTrip(request)
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
	if pool, ok := c.transport.(interface{ CloseIdleConnections() }); ok {
		pool.CloseIdleConnections()
	}
	return nil
}

type splitConn struct {
	reader io.ReadCloser
	writer io.WriteCloser
	cancel context.CancelFunc
	once   sync.Once
}

func (c *splitConn) Read(buffer []byte) (int, error)  { return c.reader.Read(buffer) }
func (c *splitConn) Write(buffer []byte) (int, error) { return c.writer.Write(buffer) }
func (c *splitConn) Close() error {
	var err error
	c.once.Do(func() { c.cancel(); err = common.Close(c.writer, c.reader) })
	return err
}
func (c *splitConn) LocalAddr() net.Addr              { return M.Socksaddr{} }
func (c *splitConn) RemoteAddr() net.Addr             { return M.Socksaddr{} }
func (c *splitConn) SetDeadline(time.Time) error      { return os.ErrInvalid }
func (c *splitConn) SetReadDeadline(time.Time) error  { return os.ErrInvalid }
func (c *splitConn) SetWriteDeadline(time.Time) error { return os.ErrInvalid }

type packetWriter struct {
	ctx       context.Context
	client    *Client
	sessionID string
	packets   chan []byte
	closed    chan struct{}
	once      sync.Once
	access    sync.Mutex
	err       error
}

func newPacketWriter(ctx context.Context, client *Client, sessionID string) *packetWriter {
	writer := &packetWriter{ctx: ctx, client: client, sessionID: sessionID, packets: make(chan []byte, client.config.maxBufferedPosts), closed: make(chan struct{})}
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
			limit := w.client.config.scMaxPost.random()
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
			if err := w.client.sendPacket(w.ctx, w.sessionID, sequence, payload); err != nil {
				w.access.Lock()
				w.err = err
				w.access.Unlock()
				return
			}
			sequence++
			interval := time.Duration(w.client.config.scMinInterval.random()) * time.Millisecond
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
