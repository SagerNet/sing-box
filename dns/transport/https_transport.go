package transport

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync/atomic"

	"github.com/sagernet/sing-box/common/tls"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"

	"golang.org/x/net/http2"
)

var errFallback = E.New("fallback to HTTP/1.1")

type HTTPSTransportWrapper struct {
	http2Transport *http2.Transport
	httpTransport  *http.Transport
	fallback       *atomic.Bool
}

func NewHTTPSTransportWrapper(dialer tls.Dialer, serverAddr M.Socksaddr) *HTTPSTransportWrapper {
	var fallback atomic.Bool
	return &HTTPSTransportWrapper{
		http2Transport: &http2.Transport{
			DialTLSContext: func(ctx context.Context, _, _ string, _ *tls.STDConfig) (net.Conn, error) {
				tlsConn, err := dialer.DialTLSContext(ctx, serverAddr)
				if err != nil {
					return nil, err
				}
				state := tlsConn.ConnectionState()
				if state.NegotiatedProtocol == http2.NextProtoTLS {
					return tlsConn, nil
				}
				tlsConn.Close()
				fallback.Store(true)
				return nil, errFallback
			},
		},
		httpTransport: &http.Transport{
			DialTLSContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return dialer.DialTLSContext(ctx, serverAddr)
			},
		},
		fallback: &fallback,
	}
}

func (h *HTTPSTransportWrapper) RoundTrip(request *http.Request) (*http.Response, error) {
	if h.fallback.Load() {
		return h.httpTransport.RoundTrip(request)
	} else {
		response, err := h.http2Transport.RoundTrip(request)
		if err != nil {
			if errors.Is(err, errFallback) {
				return h.httpTransport.RoundTrip(request)
			}
			return nil, err
		}
		return response, nil
	}
}

func (h *HTTPSTransportWrapper) CloseIdleConnections() {
	h.http2Transport.CloseIdleConnections()
	h.httpTransport.CloseIdleConnections()
}

func (h *HTTPSTransportWrapper) Clone() *HTTPSTransportWrapper {
	return &HTTPSTransportWrapper{
		httpTransport: h.httpTransport,
		http2Transport: &http2.Transport{
			DialTLSContext: h.http2Transport.DialTLSContext,
		},
		fallback: h.fallback,
	}
}
