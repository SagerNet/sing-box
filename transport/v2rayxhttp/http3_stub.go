//go:build !with_quic

package v2rayxhttp

import (
	"net"
	"net/http"

	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func validateHTTP3() error {
	return C.ErrQUICNotIncluded
}

func newHTTP3TransportFactory(N.Dialer, M.Socksaddr, tls.Config, option.QUICOptions) (func() http.RoundTripper, error) {
	return nil, C.ErrQUICNotIncluded
}

func (s *Server) ServePacket(net.PacketConn) error {
	return C.ErrQUICNotIncluded
}
