//go:build with_quic

package inbound

import (
	"net"
	"net/netip"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/http3"
	M "github.com/sagernet/sing/common/metadata"
)

func (n *Naive) configureHTTP3Listener(listenAddr string) error {
	h3Server := &http3.Server{
		Port:      int(n.listenOptions.ListenPort),
		TLSConfig: n.tlsConfig.Config(),
		Handler:   n,
	}

	udpListener, err := net.ListenPacket(M.NetworkFromNetAddr("udp", netip.Addr(n.listenOptions.Listen)), listenAddr)
	if err != nil {
		return err
	}

	n.logger.Info("udp server started at ", udpListener.LocalAddr())

	go func() {
		sErr := h3Server.Serve(udpListener)
		if sErr == quic.ErrServerClosed {
			udpListener.Close()
			return
		} else if sErr != nil {
			n.logger.Error("http3 server serve error: ", sErr)
		}
	}()

	n.h3Server = h3Server
	return nil
}
