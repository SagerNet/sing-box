//go:build !with_dhcp

package include

import (
	"github.com/sagernet/sing-dns"
	E "github.com/sagernet/sing/common/exceptions"
)

func init() {
	dns.RegisterTransport([]string{"dhcp"}, func(options dns.TransportOptions) (dns.Transport, error) {
		return nil, E.New(`DHCP is not included in this build, rebuild with -tags with_dhcp`)
	})
}
