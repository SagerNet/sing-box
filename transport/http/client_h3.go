//go:build with_quic

package http

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/http3"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/httpclient"
	"github.com/sagernet/sing-quic"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	aTLS "github.com/sagernet/sing/common/tls"
)

func init() {
	NewHTTP3Client = newHTTP3Client
}

type http3ClientImpl struct {
	dialer        N.Dialer
	tlsConfig     aTLS.Config
	server        M.Socksaddr
	authority     string
	headers       http.Header
	authorization string
	quicConfig    *quic.Config
	transport     *http3.Transport
	closeIdle     *atomic.Bool
	access        sync.Mutex
	conn          *http3Conn
	dial          *dialOffer
}

type http3Conn struct {
	*http3.ClientConn
	streams int
}

func newHTTP3Client(options ClientOptions, authorization string, closeIdle *atomic.Bool) (http3Client, error) {
	if options.TLSConfig == nil {
		return nil, E.New("HTTP/3 requires TLS")
	}
	dialer := options.RawDialer
	if dialer == nil {
		dialer = N.SystemDialer
	}
	quicConfig := httpclient.NewQUICConfig(options.HTTP3Options)
	quicConfig.EnableDatagrams = true
	headers := options.Headers.Clone()
	authority := options.Server.String()
	if options.Authority != "" {
		authority = options.Authority
	}
	if headers != nil {
		if host := headers.Get("Host"); host != "" {
			authority = host
		}
		headers.Del("Host")
	}
	return &http3ClientImpl{
		dialer:        dialer,
		tlsConfig:     options.TLSConfig,
		server:        options.Server,
		authority:     authority,
		headers:       headers,
		authorization: authorization,
		quicConfig:    quicConfig,
		transport:     &http3.Transport{EnableDatagrams: true, DisableCompression: true},
		closeIdle:     closeIdle,
	}, nil
}

func (c *http3ClientImpl) acquire(ctx context.Context) (*http3Conn, error) {
	for {
		c.access.Lock()
		if c.conn != nil && c.conn.Context().Err() == nil {
			conn := c.conn
			conn.streams++
			c.access.Unlock()
			return conn, nil
		}
		offer := c.dial
		if offer == nil {
			offer = &dialOffer{done: make(chan struct{})}
			c.dial = offer
			c.access.Unlock()
			return c.dialConn(ctx, offer)
		}
		c.access.Unlock()
		select {
		case <-offer.done:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		if offer.err != nil {
			return nil, offer.err
		}
	}
}

func (c *http3ClientImpl) dialConn(ctx context.Context, offer *dialOffer) (*http3Conn, error) {
	var quicConn *quic.Conn
	rawConn, err := c.dialer.DialContext(adapter.ContextForMultiplexSession(ctx), N.NetworkUDP, c.server)
	if err == nil {
		quicConn, err = qtls.DialEarly(ctx, rawConn, c.tlsConfig, c.quicConfig)
		if err != nil {
			rawConn.Close()
		} else {
			context.AfterFunc(quicConn.Context(), func() {
				rawConn.Close()
			})
		}
	}
	c.access.Lock()
	defer c.access.Unlock()
	defer close(offer.done)
	current := c.dial == offer
	if current {
		c.dial = nil
	}
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		err = E.Cause1(ErrHTTP3Unavailable, err)
		offer.err = err
		return nil, err
	}
	if !current {
		quicConn.CloseWithError(0, "")
		return nil, net.ErrClosed
	}
	c.conn = &http3Conn{ClientConn: c.transport.NewClientConn(quicConn), streams: 1}
	return c.conn, nil
}

func (c *http3ClientImpl) release(conn *http3Conn, keepSession bool) {
	c.access.Lock()
	defer c.access.Unlock()
	conn.streams--
	if conn.streams == 0 && !keepSession && c.closeIdle.Load() {
		conn.CloseWithError(0, "")
		if c.conn == conn {
			c.conn = nil
		}
	}
}

func (c *http3ClientImpl) openRequestStream(ctx context.Context, extendedConnect bool, keepSession bool) (*http3Conn, *http3.RequestStream, error) {
	for retry := true; ; retry = false {
		conn, err := c.acquire(ctx)
		if err != nil {
			return nil, nil, err
		}
		if extendedConnect {
			select {
			case <-conn.ReceivedSettings():
				if !conn.Settings().EnableExtendedConnect {
					err = errHTTP3ExtendedConnectUnsupported
				}
			case <-conn.Context().Done():
				err = E.Cause1(ErrHTTP3Unavailable, context.Cause(conn.Context()))
			case <-ctx.Done():
				err = E.Cause(ctx.Err(), "wait for HTTP/3 SETTINGS")
			}
		}
		if err == nil {
			var stream *http3.RequestStream
			stream, err = conn.OpenRequestStream(ctx)
			if err == nil {
				return conn, stream, nil
			}
			if retry && errors.Is(err, http3.ErrNoMoreStreams) {
				c.access.Lock()
				if c.conn == conn {
					c.conn = nil
				}
				c.access.Unlock()
				c.release(conn, keepSession)
				continue
			}
			if ctx.Err() != nil {
				err = ctx.Err()
			} else {
				err = E.Cause1(ErrHTTP3Unavailable, err)
			}
		}
		c.release(conn, keepSession)
		return nil, nil, err
	}
}

func (c *http3ClientImpl) openStream(ctx context.Context, request *http.Request, extendedConnect bool, keepSession bool) (*http3.RequestStream, *http3Conn, error) {
	conn, stream, err := c.openRequestStream(ctx, extendedConnect, keepSession)
	if err != nil {
		return nil, nil, err
	}
	stop := context.AfterFunc(ctx, func() {
		stream.CancelRead(0)
		stream.CancelWrite(0)
	})
	var response *http.Response
	err = stream.SendRequestHeader(request)
	if err == nil {
		response, err = stream.ReadResponse()
	}
	if !stop() {
		err = ctx.Err()
	}
	if err != nil {
		stream.CancelRead(0)
		stream.Close()
		c.release(conn, keepSession)
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return nil, nil, E.Cause(err, "HTTP/3 CONNECT")
	}
	if response.StatusCode != http.StatusOK {
		stream.CancelRead(0)
		stream.Close()
		c.release(conn, keepSession)
		return nil, nil, statusError(response)
	}
	return stream, conn, nil
}

func (c *http3ClientImpl) DialContext(ctx context.Context, destination M.Socksaddr) (net.Conn, error) {
	keepSession := adapter.KeepSessionFromContext(ctx)
	stream, conn, err := c.openStream(ctx, &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Host: destination.String()},
		Host:   destination.String(),
		Header: buildRequestHeader(c.headers, c.authorization, false),
	}, false, keepSession)
	if err != nil {
		return nil, err
	}
	return &http3StreamConn{
		stream:     stream,
		remoteAddr: destination,
		onClose:    func() { c.release(conn, keepSession) },
	}, nil
}

func (c *http3ClientImpl) OpenTunnel(ctx context.Context, request tunnelRequest) (DatagramStream, error) {
	requestURL := *request.url
	requestURL.Scheme = "https"
	requestURL.Host = c.authority
	header := buildRequestHeader(c.headers, c.authorization, request.originAuthorization)
	header.Set("Capsule-Protocol", "?1")
	keepSession := adapter.KeepSessionFromContext(ctx)
	stream, conn, err := c.openStream(ctx, &http.Request{
		Method: http.MethodConnect,
		Proto:  request.protocol,
		URL:    &requestURL,
		Host:   c.authority,
		Header: header,
	}, true, keepSession)
	if err != nil {
		return nil, err
	}
	return &http3RequestDatagramStream{
		stream:           stream,
		datagramsEnabled: conn.Settings().EnableDatagrams,
		onClose:          func() { c.release(conn, keepSession) },
	}, nil
}

func (c *http3ClientImpl) CloseIdleConnections() {
	c.access.Lock()
	defer c.access.Unlock()
	if c.conn != nil && c.conn.streams == 0 {
		c.conn.CloseWithError(0, "")
		c.conn = nil
	}
}

func (c *http3ClientImpl) ResetConnection() {
	c.access.Lock()
	defer c.access.Unlock()
	c.dial = nil
	if c.conn != nil {
		c.conn.CloseWithError(0, "")
		c.conn = nil
	}
}

func (c *http3ClientImpl) Close() error {
	c.ResetConnection()
	return c.transport.Close()
}

type http3StreamConn struct {
	stream      *http3.RequestStream
	remoteAddr  net.Addr
	onClose     func()
	writeAccess sync.Mutex
	closed      atomic.Bool
}

func (c *http3StreamConn) Read(p []byte) (int, error) {
	n, err := c.stream.Read(p)
	return n, c.wrapError(err)
}

func (c *http3StreamConn) Write(p []byte) (int, error) {
	c.writeAccess.Lock()
	defer c.writeAccess.Unlock()
	n, err := c.stream.Write(p)
	return n, c.wrapError(err)
}

func (c *http3StreamConn) wrapError(err error) error {
	if err == nil {
		return nil
	}
	if c.closed.Load() {
		return net.ErrClosed
	}
	return qtls.WrapError(err)
}

func (c *http3StreamConn) CloseWrite() error {
	c.writeAccess.Lock()
	defer c.writeAccess.Unlock()
	return c.stream.Close()
}

func (c *http3StreamConn) Close() error {
	if c.closed.Swap(true) {
		return nil
	}
	c.stream.SetWriteDeadline(time.Now())
	c.writeAccess.Lock()
	c.stream.CancelRead(0)
	err := c.stream.Close()
	c.writeAccess.Unlock()
	c.onClose()
	return err
}

func (c *http3StreamConn) LocalAddr() net.Addr {
	return M.Socksaddr{}
}

func (c *http3StreamConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *http3StreamConn) SetDeadline(t time.Time) error {
	return c.stream.SetDeadline(t)
}

func (c *http3StreamConn) SetReadDeadline(t time.Time) error {
	return c.stream.SetReadDeadline(t)
}

func (c *http3StreamConn) SetWriteDeadline(t time.Time) error {
	return c.stream.SetWriteDeadline(t)
}

func (c *http3StreamConn) NeedAdditionalReadDeadline() bool {
	return true
}

type http3RequestDatagramStream struct {
	stream           *http3.RequestStream
	datagramsEnabled bool
	onClose          func()
	closed           atomic.Bool
}

func (s *http3RequestDatagramStream) Read(p []byte) (int, error) {
	return s.stream.Read(p)
}

func (s *http3RequestDatagramStream) Write(p []byte) (int, error) {
	return s.stream.Write(p)
}

func (s *http3RequestDatagramStream) Close() error {
	if s.closed.Swap(true) {
		return nil
	}
	s.stream.SetWriteDeadline(time.Now())
	s.stream.CancelRead(0)
	err := s.stream.Close()
	s.onClose()
	return err
}

func (s *http3RequestDatagramStream) SendDatagram(payload []byte) error {
	if !s.datagramsEnabled {
		return ErrDatagramUnsupported
	}
	err := s.stream.SendDatagram(payload)
	if err == nil {
		return nil
	}
	var tooLarge *quic.DatagramTooLargeError
	if errors.As(err, &tooLarge) {
		return &DatagramTooLargeError{MaxPayloadSize: int(tooLarge.MaxDatagramPayloadSize) - VarintLen(uint64(s.stream.StreamID()/4))}
	}
	return err
}

func (s *http3RequestDatagramStream) ReceiveDatagram(ctx context.Context) ([]byte, error) {
	return s.stream.ReceiveDatagram(ctx)
}

var (
	_ net.Conn       = (*http3StreamConn)(nil)
	_ N.WriteCloser  = (*http3StreamConn)(nil)
	_ DatagramStream = (*http3RequestDatagramStream)(nil)
)
