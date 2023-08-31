//go:build with_quic

package inbound

import (
	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/http3"
	"github.com/sagernet/sing-box/common/qtls"
	E "github.com/sagernet/sing/common/exceptions"
)

func (n *Naive) configureHTTP3Listener() error {
	err := qtls.ConfigureHTTP3(n.tlsConfig)
	if err != nil {
		return err
	}

	udpConn, err := n.ListenUDP()
	if err != nil {
		return err
	}

	quicListener, err := qtls.ListenEarly(udpConn, n.tlsConfig, &quic.Config{
		MaxIncomingStreams: 1 << 60,
		Allow0RTT:          true,
	})
	if err != nil {
		udpConn.Close()
		return err
	}

	h3Server := &http3.Server{
		Port:    int(n.listenOptions.ListenPort),
		Handler: n,
	}

	go func() {
		sErr := h3Server.ServeListener(quicListener)
		udpConn.Close()
		if sErr != nil && !E.IsClosedOrCanceled(sErr) {
			n.logger.Error("http3 server serve error: ", sErr)
		}
	}()

	n.h3Server = h3Server
	return nil
}
