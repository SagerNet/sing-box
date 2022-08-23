//go:build with_quic

package inbound

import (
	"github.com/sagernet/quic-go/http3"
	E "github.com/sagernet/sing/common/exceptions"
)

func (n *Naive) configureHTTP3Listener() error {
	h3Server := &http3.Server{
		Port:      int(n.listenOptions.ListenPort),
		TLSConfig: n.tlsConfig.Config(),
		Handler:   n,
	}

	udpConn, err := n.ListenUDP()
	if err != nil {
		return err
	}

	go func() {
		sErr := h3Server.Serve(udpConn)
		udpConn.Close()
		if sErr != nil && !E.IsClosedOrCanceled(sErr) {
			n.logger.Error("http3 server serve error: ", sErr)
		}
	}()

	n.h3Server = h3Server
	return nil
}
