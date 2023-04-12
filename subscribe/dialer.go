package subscribe

import (
	D "github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/option"
	N "github.com/sagernet/sing/common/network"
)

func NewDialer(options option.RequestDialerOptions) N.Dialer {
	opt := option.DialerOptions{
		BindInterface:      options.BindInterface,
		Inet4BindAddress:   options.Inet4BindAddress,
		Inet6BindAddress:   options.Inet6BindAddress,
		ProtectPath:        options.ProtectPath,
		RoutingMark:        options.RoutingMark,
		ReuseAddr:          options.ReuseAddr,
		ConnectTimeout:     options.ConnectTimeout,
		TCPFastOpen:        options.TCPFastOpen,
		UDPFragment:        options.UDPFragment,
		UDPFragmentDefault: options.UDPFragmentDefault,
	}
	return D.NewSimple(opt)
}
