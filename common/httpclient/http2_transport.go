package httpclient

import (
	"context"
	stdTLS "crypto/tls"
	"net"
	"net/http"

	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"golang.org/x/net/http2"
)

type http2Transport struct {
	h2Transport *http2.Transport
	h1Transport *http1Transport
}

func newHTTP2Transport(rawDialer N.Dialer, baseTLSConfig tls.Config, options option.HTTP2Options) (*http2Transport, error) {
	h1 := newHTTP1Transport(rawDialer, baseTLSConfig)
	h2Transport, err := ConfigureHTTP2Transport(options)
	if err != nil {
		return nil, err
	}
	h2Transport.DialTLSContext = func(ctx context.Context, network, addr string, _ *stdTLS.Config) (net.Conn, error) {
		return dialTLS(ctx, rawDialer, baseTLSConfig, M.ParseSocksaddr(addr), []string{http2.NextProtoTLS}, http2.NextProtoTLS)
	}
	return &http2Transport{
		h2Transport: h2Transport,
		h1Transport: h1,
	}, nil
}

func (t *http2Transport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.Scheme != "https" || requestRequiresHTTP1(request) {
		return t.h1Transport.RoundTrip(request)
	}
	return t.h2Transport.RoundTrip(request)
}

func (t *http2Transport) CloseIdleConnections() {
	t.h1Transport.CloseIdleConnections()
	t.h2Transport.CloseIdleConnections()
}

func (t *http2Transport) Close() error {
	t.CloseIdleConnections()
	return nil
}
