//go:build go1.20

package dialer

import (
	"net"
)

type tcpDialer = ExtendedTCPDialer

func newTCPDialer(dialer net.Dialer, tfoEnabled bool, tlsFragment TLSFragment) (tcpDialer, error) {
	return tcpDialer{Dialer: dialer, DisableTFO: !tfoEnabled, TLSFragment: tlsFragment}, nil
}

func dialerFromTCPDialer(dialer tcpDialer) net.Dialer {
	return dialer.Dialer
}
