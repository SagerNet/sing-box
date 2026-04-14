//go:build with_quic

package httpclient

import (
	"context"
	stdTLS "crypto/tls"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/http3"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type http3Transport struct {
	h3Transport *http3.Transport
}

type http3FallbackTransport struct {
	h3Transport   *http3.Transport
	h2Fallback    innerTransport
	fallbackDelay time.Duration
	brokenAccess  sync.Mutex
	brokenUntil   time.Time
	brokenBackoff time.Duration
}

func newHTTP3RoundTripper(
	rawDialer N.Dialer,
	baseTLSConfig tls.Config,
	options option.QUICOptions,
) *http3.Transport {
	var handshakeTimeout time.Duration
	if baseTLSConfig != nil {
		handshakeTimeout = baseTLSConfig.HandshakeTimeout()
	}
	quicConfig := &quic.Config{
		InitialStreamReceiveWindow:     options.StreamReceiveWindow.Value(),
		MaxStreamReceiveWindow:         options.StreamReceiveWindow.Value(),
		InitialConnectionReceiveWindow: options.ConnectionReceiveWindow.Value(),
		MaxConnectionReceiveWindow:     options.ConnectionReceiveWindow.Value(),
		KeepAlivePeriod:                time.Duration(options.KeepAlivePeriod),
		MaxIdleTimeout:                 time.Duration(options.IdleTimeout),
		DisablePathMTUDiscovery:        options.DisablePathMTUDiscovery,
	}
	if options.InitialPacketSize > 0 {
		quicConfig.InitialPacketSize = uint16(options.InitialPacketSize)
	}
	if options.MaxConcurrentStreams > 0 {
		quicConfig.MaxIncomingStreams = int64(options.MaxConcurrentStreams)
	}
	if handshakeTimeout > 0 {
		quicConfig.HandshakeIdleTimeout = handshakeTimeout
	}
	h3Transport := &http3.Transport{
		TLSClientConfig: &stdTLS.Config{},
		QUICConfig:      quicConfig,
		Dial: func(ctx context.Context, addr string, tlsConfig *stdTLS.Config, quicConfig *quic.Config) (*quic.Conn, error) {
			if handshakeTimeout > 0 && quicConfig.HandshakeIdleTimeout == 0 {
				quicConfig = quicConfig.Clone()
				quicConfig.HandshakeIdleTimeout = handshakeTimeout
			}
			if baseTLSConfig != nil {
				var err error
				tlsConfig, err = buildSTDTLSConfig(baseTLSConfig, M.ParseSocksaddr(addr), []string{http3.NextProtoH3})
				if err != nil {
					return nil, err
				}
			} else {
				tlsConfig = tlsConfig.Clone()
				tlsConfig.NextProtos = []string{http3.NextProtoH3}
			}
			conn, err := rawDialer.DialContext(ctx, N.NetworkUDP, M.ParseSocksaddr(addr))
			if err != nil {
				return nil, err
			}
			quicConn, err := quic.DialEarly(ctx, bufio.NewUnbindPacketConn(conn), conn.RemoteAddr(), tlsConfig, quicConfig)
			if err != nil {
				conn.Close()
				return nil, err
			}
			return quicConn, nil
		},
	}
	return h3Transport
}

func newHTTP3Transport(
	rawDialer N.Dialer,
	baseTLSConfig tls.Config,
	options option.QUICOptions,
) (innerTransport, error) {
	return &http3Transport{
		h3Transport: newHTTP3RoundTripper(rawDialer, baseTLSConfig, options),
	}, nil
}

func newHTTP3FallbackTransport(
	rawDialer N.Dialer,
	baseTLSConfig tls.Config,
	h2Fallback innerTransport,
	options option.QUICOptions,
	fallbackDelay time.Duration,
) (innerTransport, error) {
	return &http3FallbackTransport{
		h3Transport:   newHTTP3RoundTripper(rawDialer, baseTLSConfig, options),
		h2Fallback:    h2Fallback,
		fallbackDelay: fallbackDelay,
	}, nil
}

func (t *http3Transport) RoundTrip(request *http.Request) (*http.Response, error) {
	return t.h3Transport.RoundTrip(request)
}

func (t *http3Transport) CloseIdleConnections() {
	t.h3Transport.CloseIdleConnections()
}

func (t *http3Transport) Close() error {
	t.CloseIdleConnections()
	return t.h3Transport.Close()
}

func (t *http3FallbackTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.Scheme != "https" || requestRequiresHTTP1(request) {
		return t.h2Fallback.RoundTrip(request)
	}
	return t.roundTripHTTP3(request)
}

func (t *http3FallbackTransport) roundTripHTTP3(request *http.Request) (*http.Response, error) {
	if t.h3Broken() {
		return t.h2FallbackRoundTrip(request)
	}
	response, err := t.h3Transport.RoundTripOpt(request, http3.RoundTripOpt{OnlyCachedConn: true})
	if err == nil {
		t.clearH3Broken()
		return response, nil
	}
	if !errors.Is(err, http3.ErrNoCachedConn) {
		t.markH3Broken()
		return t.h2FallbackRoundTrip(cloneRequestForRetry(request))
	}
	if !requestReplayable(request) {
		response, err = t.h3Transport.RoundTrip(request)
		if err == nil {
			t.clearH3Broken()
			return response, nil
		}
		t.markH3Broken()
		return nil, err
	}
	return t.roundTripHTTP3Race(request)
}

func (t *http3FallbackTransport) roundTripHTTP3Race(request *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithCancel(request.Context())
	defer cancel()
	type result struct {
		response *http.Response
		err      error
		h3       bool
	}
	results := make(chan result, 2)
	startRoundTrip := func(request *http.Request, useH3 bool) {
		request = request.WithContext(ctx)
		var (
			response *http.Response
			err      error
		)
		if useH3 {
			response, err = t.h3Transport.RoundTrip(request)
		} else {
			response, err = t.h2FallbackRoundTrip(request)
		}
		results <- result{response: response, err: err, h3: useH3}
	}
	goroutines := 1
	received := 0
	drainRemaining := func() {
		cancel()
		for range goroutines - received {
			go func() {
				loser := <-results
				if loser.response != nil && loser.response.Body != nil {
					loser.response.Body.Close()
				}
			}()
		}
	}
	go startRoundTrip(cloneRequestForRetry(request), true)
	timer := time.NewTimer(t.fallbackDelay)
	defer timer.Stop()
	var (
		h3Err       error
		fallbackErr error
	)
	for {
		select {
		case <-timer.C:
			if goroutines == 1 {
				goroutines++
				go startRoundTrip(cloneRequestForRetry(request), false)
			}
		case raceResult := <-results:
			received++
			if raceResult.err == nil {
				if raceResult.h3 {
					t.clearH3Broken()
				}
				drainRemaining()
				return raceResult.response, nil
			}
			if raceResult.h3 {
				t.markH3Broken()
				h3Err = raceResult.err
				if goroutines == 1 {
					goroutines++
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					go startRoundTrip(cloneRequestForRetry(request), false)
				}
			} else {
				fallbackErr = raceResult.err
			}
			if received < goroutines {
				continue
			}
			drainRemaining()
			switch {
			case h3Err != nil && fallbackErr != nil:
				return nil, E.Errors(h3Err, fallbackErr)
			case fallbackErr != nil:
				return nil, fallbackErr
			default:
				return nil, h3Err
			}
		}
	}
}

func (t *http3FallbackTransport) h2FallbackRoundTrip(request *http.Request) (*http.Response, error) {
	if fallback, isFallback := t.h2Fallback.(*http2FallbackTransport); isFallback {
		return fallback.roundTrip(request, true)
	}
	return t.h2Fallback.RoundTrip(request)
}

func (t *http3FallbackTransport) CloseIdleConnections() {
	t.h3Transport.CloseIdleConnections()
	t.h2Fallback.CloseIdleConnections()
}

func (t *http3FallbackTransport) Close() error {
	t.CloseIdleConnections()
	return t.h3Transport.Close()
}

func (t *http3FallbackTransport) h3Broken() bool {
	t.brokenAccess.Lock()
	defer t.brokenAccess.Unlock()
	return !t.brokenUntil.IsZero() && time.Now().Before(t.brokenUntil)
}

func (t *http3FallbackTransport) clearH3Broken() {
	t.brokenAccess.Lock()
	t.brokenUntil = time.Time{}
	t.brokenBackoff = 0
	t.brokenAccess.Unlock()
}

func (t *http3FallbackTransport) markH3Broken() {
	t.brokenAccess.Lock()
	defer t.brokenAccess.Unlock()
	if t.brokenBackoff == 0 {
		t.brokenBackoff = 5 * time.Minute
	} else {
		t.brokenBackoff *= 2
		if t.brokenBackoff > 48*time.Hour {
			t.brokenBackoff = 48 * time.Hour
		}
	}
	t.brokenUntil = time.Now().Add(t.brokenBackoff)
}
