//go:build with_quic

package main

import (
	"context"
	"crypto/tls"
	"net/http"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/http3"
	box "github.com/sagernet/sing-box"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func initializeHTTP3Client(instance *box.Box) error {
	dialer, err := createDialer(instance, commandToolsFlagOutbound)
	if err != nil {
		return err
	}
	http3Client = &http.Client{
		Transport: &http3.Transport{
			Dial: func(ctx context.Context, addr string, tlsCfg *tls.Config, cfg *quic.Config) (*quic.Conn, error) {
				destination := M.ParseSocksaddr(addr)
				udpConn, dErr := dialer.DialContext(ctx, N.NetworkUDP, destination)
				if dErr != nil {
					return nil, dErr
				}
				quicConn, dErr := quic.DialEarlyConn(ctx, udpConn, tlsCfg, cfg)
				if dErr != nil {
					udpConn.Close()
					return nil, dErr
				}
				go func() {
					<-quicConn.Context().Done()
					udpConn.Close()
				}()
				return quicConn, nil
			},
		},
	}
	return nil
}
