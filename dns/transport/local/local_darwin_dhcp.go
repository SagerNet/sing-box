//go:build darwin && with_dhcp

package local

import (
	"context"

	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/dns/transport/dhcp"
	"github.com/sagernet/sing-box/log"
	N "github.com/sagernet/sing/common/network"
)

func newDHCPTransport(transportAdapter dns.TransportAdapter, ctx context.Context, dialer N.Dialer, logger log.ContextLogger) dhcpTransport {
	return dhcp.NewRawTransport(transportAdapter, ctx, dialer, logger)
}
