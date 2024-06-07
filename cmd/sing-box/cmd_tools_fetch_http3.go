//go:build with_quic

package main

import (
	"context"
	"crypto/tls"
	"net/http"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/http3"
	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func initializeHTTP3Client(instance *box.Box) error {
	dialer, err := createDialer(instance, N.NetworkUDP, commandToolsFlagOutbound)
	if err != nil {
		return err
	}
	http3Client = &http.Client{
		Transport: &http3.RoundTripper{
			Dial: func(ctx context.Context, addr string, tlsCfg *tls.Config, cfg *quic.Config) (quic.EarlyConnection, error) {
				destination := M.ParseSocksaddr(addr)
				udpConn, dErr := dialer.DialContext(ctx, N.NetworkUDP, destination)
				if dErr != nil {
					return nil, dErr
				}
				return quic.DialEarly(ctx, bufio.NewUnbindPacketConn(udpConn), udpConn.RemoteAddr(), tlsCfg, cfg)
			},
		},
	}
	return nil
}
