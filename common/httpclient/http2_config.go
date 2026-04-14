package httpclient

import (
	stdTLS "crypto/tls"
	"net/http"
	"time"

	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/net/http2"
)

func CloneHTTP2Transport(transport *http2.Transport) *http2.Transport {
	return &http2.Transport{
		ReadIdleTimeout: transport.ReadIdleTimeout,
		PingTimeout:     transport.PingTimeout,
		DialTLSContext:  transport.DialTLSContext,
	}
}

func ConfigureHTTP2Transport(options option.HTTP2Options) (*http2.Transport, error) {
	stdTransport := &http.Transport{
		TLSClientConfig: &stdTLS.Config{},
		HTTP2: &http.HTTP2Config{
			MaxReceiveBufferPerStream:     int(options.StreamReceiveWindow.Value()),
			MaxReceiveBufferPerConnection: int(options.ConnectionReceiveWindow.Value()),
			MaxConcurrentStreams:          options.MaxConcurrentStreams,
			SendPingTimeout:               time.Duration(options.KeepAlivePeriod),
			PingTimeout:                   time.Duration(options.IdleTimeout),
		},
	}
	h2Transport, err := http2.ConfigureTransports(stdTransport)
	if err != nil {
		return nil, E.Cause(err, "configure HTTP/2 transport")
	}
	// ConfigureTransports binds ConnPool to the throwaway http.Transport; sever it so DialTLSContext is used directly.
	h2Transport.ConnPool = nil
	h2Transport.ReadIdleTimeout = time.Duration(options.KeepAlivePeriod)
	h2Transport.PingTimeout = time.Duration(options.IdleTimeout)
	return h2Transport, nil
}
