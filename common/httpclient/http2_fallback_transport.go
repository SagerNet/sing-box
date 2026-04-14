package httpclient

import (
	"context"
	stdTLS "crypto/tls"
	"errors"
	"net"
	"net/http"
	"sync/atomic"

	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"golang.org/x/net/http2"
)

var errHTTP2Fallback = E.New("fallback to HTTP/1.1")

type http2FallbackTransport struct {
	h2Transport *http2.Transport
	h1Transport *http1Transport
	h2Fallback  *atomic.Bool
}

func newHTTP2FallbackTransport(rawDialer N.Dialer, baseTLSConfig tls.Config, options option.HTTP2Options) (*http2FallbackTransport, error) {
	h1 := newHTTP1Transport(rawDialer, baseTLSConfig)
	var fallback atomic.Bool
	h2Transport, err := ConfigureHTTP2Transport(options)
	if err != nil {
		return nil, err
	}
	h2Transport.DialTLSContext = func(ctx context.Context, network, addr string, _ *stdTLS.Config) (net.Conn, error) {
		conn, dialErr := dialTLS(ctx, rawDialer, baseTLSConfig, M.ParseSocksaddr(addr), []string{http2.NextProtoTLS, "http/1.1"}, http2.NextProtoTLS)
		if dialErr != nil {
			if errors.Is(dialErr, errHTTP2Fallback) {
				fallback.Store(true)
			}
			return nil, dialErr
		}
		return conn, nil
	}
	return &http2FallbackTransport{
		h2Transport: h2Transport,
		h1Transport: h1,
		h2Fallback:  &fallback,
	}, nil
}

func (t *http2FallbackTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return t.roundTrip(request, true)
}

func (t *http2FallbackTransport) roundTrip(request *http.Request, allowHTTP1Fallback bool) (*http.Response, error) {
	if request.URL.Scheme != "https" || requestRequiresHTTP1(request) {
		return t.h1Transport.RoundTrip(request)
	}
	if t.h2Fallback.Load() {
		if !allowHTTP1Fallback {
			return nil, errHTTP2Fallback
		}
		return t.h1Transport.RoundTrip(request)
	}
	response, err := t.h2Transport.RoundTrip(request)
	if err == nil {
		return response, nil
	}
	if !errors.Is(err, errHTTP2Fallback) || !allowHTTP1Fallback {
		return nil, err
	}
	return t.h1Transport.RoundTrip(cloneRequestForRetry(request))
}

func (t *http2FallbackTransport) CloseIdleConnections() {
	t.h1Transport.CloseIdleConnections()
	t.h2Transport.CloseIdleConnections()
}

func (t *http2FallbackTransport) Close() error {
	t.CloseIdleConnections()
	return nil
}
