//go:build with_quic

package networkquality

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/http3"
	sBufio "github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func NewHTTP3MeasurementClientFactory(dialer N.Dialer) (MeasurementClientFactory, error) {
	// singleConnection and disableKeepAlives are not applied:
	// HTTP/3 multiplexes streams over a single QUIC connection by default.
	return func(connectEndpoint string, _, _ bool, readCounters, writeCounters []N.CountFunc) (*http.Client, error) {
		transport := &http3.Transport{
			Dial: func(ctx context.Context, addr string, tlsCfg *tls.Config, cfg *quic.Config) (*quic.Conn, error) {
				dialAddr := addr
				if connectEndpoint != "" {
					dialAddr = rewriteDialAddress(addr, connectEndpoint)
				}
				destination := M.ParseSocksaddr(dialAddr)
				var udpConn net.Conn
				var dialErr error
				if dialer != nil {
					udpConn, dialErr = dialer.DialContext(ctx, N.NetworkUDP, destination)
				} else {
					var netDialer net.Dialer
					udpConn, dialErr = netDialer.DialContext(ctx, N.NetworkUDP, destination.String())
				}
				if dialErr != nil {
					return nil, dialErr
				}
				var wrappedConn net.Conn = udpConn
				if len(readCounters) > 0 || len(writeCounters) > 0 {
					wrappedConn = sBufio.NewCounterConn(udpConn, readCounters, writeCounters)
				}
				packetConn := sBufio.NewUnbindPacketConn(wrappedConn)
				quicConn, dialErr := quic.DialEarly(ctx, packetConn, udpConn.RemoteAddr(), tlsCfg, cfg)
				if dialErr != nil {
					udpConn.Close()
					return nil, dialErr
				}
				return quicConn, nil
			},
		}
		return &http.Client{Transport: transport}, nil
	}, nil
}
