//go:build !with_quic

package inbound

import E "github.com/sagernet/sing/common/exceptions"

func (n *Naive) configureHTTP3Listener(listenAddr string) error {
	return E.New("QUIC is not included in this build, rebuild with -tags with_quic")
}
