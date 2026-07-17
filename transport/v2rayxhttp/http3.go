//go:build with_quic

package v2rayxhttp

import (
	"context"
	"net"
	"net/http"
	"runtime"
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

func validateHTTP3() error {
	return nil
}

func newHTTP3TransportFactory(dialer N.Dialer, serverAddr M.Socksaddr, tlsConfig tls.Config, options option.QUICOptions) (func() http.RoundTripper, error) {
	stdTLSConfig, err := tlsConfig.STDConfig()
	if err != nil {
		return nil, E.Cause(err, "xhttp HTTP/3 requires standard TLS")
	}
	stdTLSConfig = stdTLSConfig.Clone()
	stdTLSConfig.NextProtos = []string{http3.NextProtoH3}
	return func() http.RoundTripper {
		clientTLSConfig := stdTLSConfig.Clone()
		return &http3.Transport{
			TLSClientConfig: clientTLSConfig,
			QUICConfig:      newHTTP3QUICConfig(options),
			Dial: func(ctx context.Context, _ string, _ *tls.STDConfig, config *quic.Config) (*quic.Conn, error) {
				connection, err := dialer.DialContext(ctx, N.NetworkUDP, serverAddr)
				if err != nil {
					return nil, err
				}
				packetConn := bufio.NewUnbindPacketConn(connection)
				quicConn, err := quic.DialEarly(ctx, packetConn, connection.RemoteAddr(), clientTLSConfig, config)
				if err != nil {
					_ = packetConn.Close()
					return nil, err
				}
				context.AfterFunc(quicConn.Context(), func() { _ = packetConn.Close() })
				return quicConn, nil
			},
		}
	}, nil
}

func newHTTP3QUICConfig(options option.QUICOptions) *quic.Config {
	config := &quic.Config{
		InitialStreamReceiveWindow:     options.StreamReceiveWindow.Value(),
		MaxStreamReceiveWindow:         options.StreamReceiveWindow.Value(),
		InitialConnectionReceiveWindow: options.ConnectionReceiveWindow.Value(),
		MaxConnectionReceiveWindow:     options.ConnectionReceiveWindow.Value(),
		KeepAlivePeriod:                time.Duration(options.KeepAlivePeriod),
		MaxIdleTimeout:                 time.Duration(options.IdleTimeout),
		DisablePathMTUDiscovery:        options.DisablePathMTUDiscovery || (runtime.GOOS != "linux" && runtime.GOOS != "windows" && runtime.GOOS != "darwin"),
	}
	if options.InitialPacketSize > 0 {
		config.InitialPacketSize = uint16(options.InitialPacketSize)
	}
	if options.MaxConcurrentStreams > 0 {
		config.MaxIncomingStreams = int64(options.MaxConcurrentStreams)
	}
	return config
}

func (s *Server) ServePacket(listener net.PacketConn) error {
	stdTLSConfig, err := s.tlsConfig.STDConfig()
	if err != nil {
		return E.Cause(err, "xhttp HTTP/3 requires standard TLS")
	}
	stdTLSConfig = stdTLSConfig.Clone()
	stdTLSConfig.NextProtos = []string{http3.NextProtoH3}
	server := &http3.Server{
		Handler:    s,
		TLSConfig:  stdTLSConfig,
		QUICConfig: newHTTP3QUICConfig(s.config.quic),
	}
	s.packetServer = server
	return server.Serve(listener)
}
