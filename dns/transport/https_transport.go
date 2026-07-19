package transport

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/sagernet/sing-box/common/tls"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"golang.org/x/net/http2"
)

var errFallback = E.New("fallback to HTTP/1.1")

type HTTPSTransportWrapper struct {
	http2Transport *http2.Transport
	httpTransport  *http.Transport
	fallback       *atomic.Bool
	connAccess     sync.Mutex
	connections    map[*httpsTrackedConn]struct{}
	closed         bool
}

func NewHTTPSTransportWrapper(dialer N.Dialer, serverAddr M.Socksaddr, fallback *atomic.Bool) *HTTPSTransportWrapper {
	wrapper := &HTTPSTransportWrapper{
		fallback:    fallback,
		connections: make(map[*httpsTrackedConn]struct{}),
	}
	wrapper.http2Transport = &http2.Transport{
		DialTLSContext: func(ctx context.Context, _, _ string, _ *tls.STDConfig) (net.Conn, error) {
			resultConn, err := dialer.DialContext(ctx, N.NetworkTCP, serverAddr)
			if err != nil {
				return nil, err
			}
			if tlsConn, isTLSConn := resultConn.(tls.Conn); isTLSConn {
				state := tlsConn.ConnectionState()
				if state.NegotiatedProtocol != http2.NextProtoTLS {
					tlsConn.Close()
					fallback.Store(true)
					return nil, errFallback
				}
			}
			return wrapper.trackConn(resultConn)
		},
	}
	wrapper.httpTransport = &http.Transport{
		DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
			resultConn, err := dialer.DialContext(ctx, N.NetworkTCP, serverAddr)
			if err != nil {
				return nil, err
			}
			return wrapper.trackConn(resultConn)
		},
		DialTLSContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			resultConn, err := dialer.DialContext(ctx, N.NetworkTCP, serverAddr)
			if err != nil {
				return nil, err
			}
			return wrapper.trackConn(resultConn)
		},
	}
	return wrapper
}

func (h *HTTPSTransportWrapper) RoundTrip(request *http.Request) (*http.Response, error) {
	if h.fallback.Load() {
		return h.httpTransport.RoundTrip(request)
	}
	response, err := h.http2Transport.RoundTrip(request)
	if err != nil {
		if errors.Is(err, errFallback) {
			return h.httpTransport.RoundTrip(request)
		}
		return nil, err
	}
	return response, nil
}

func (h *HTTPSTransportWrapper) trackConn(conn net.Conn) (net.Conn, error) {
	trackedConn := &httpsTrackedConn{Conn: conn, wrapper: h}
	h.connAccess.Lock()
	if h.closed {
		h.connAccess.Unlock()
		conn.Close()
		return nil, net.ErrClosed
	}
	h.connections[trackedConn] = struct{}{}
	h.connAccess.Unlock()
	return trackedConn, nil
}

func (h *HTTPSTransportWrapper) Close() {
	h.connAccess.Lock()
	if h.closed {
		h.connAccess.Unlock()
		return
	}
	h.closed = true
	connections := make([]*httpsTrackedConn, 0, len(h.connections))
	for trackedConn := range h.connections {
		connections = append(connections, trackedConn)
	}
	h.connections = nil
	h.connAccess.Unlock()
	for _, trackedConn := range connections {
		trackedConn.Conn.Close()
	}
	h.http2Transport.CloseIdleConnections()
	h.httpTransport.CloseIdleConnections()
}

type httpsTrackedConn struct {
	net.Conn
	wrapper *HTTPSTransportWrapper
}

func (c *httpsTrackedConn) Close() error {
	c.wrapper.connAccess.Lock()
	delete(c.wrapper.connections, c)
	c.wrapper.connAccess.Unlock()
	return c.Conn.Close()
}
