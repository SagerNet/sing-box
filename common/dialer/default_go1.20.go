//go:build go1.20

package dialer

import (
	"net"

	"github.com/metacubex/tfo-go"
)

type tcpDialer = tfo.Dialer

func newTCPDialer(dialer net.Dialer, tfoEnabled bool) (tcpDialer, error) {
	return tfo.Dialer{Dialer: dialer, DisableTFO: !tfoEnabled}, nil
}
