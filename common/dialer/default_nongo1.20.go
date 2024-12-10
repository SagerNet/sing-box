//go:build !go1.20

package dialer

import (
	"net"

	E "github.com/sagernet/sing/common/exceptions"
)

type tcpDialer = net.Dialer

func newTCPDialer(dialer net.Dialer, tfoEnabled bool, tlsFragment TLSFragment) (tcpDialer, error) {
	if tfoEnabled {
		return dialer, E.New("TCP Fast Open requires go1.20, please recompile your binary.")
	}
	if tlsFragment.Enabled {
		return tcpDialer{Dialer: dialer, DisableTFO: true, TLSFragment: tlsFragment}, nil
	}
	return dialer, nil
}

func dialerFromTCPDialer(dialer tcpDialer) net.Dialer {
	return dialer
}
