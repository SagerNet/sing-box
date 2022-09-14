//go:build !with_quic

package include

import (
	"context"

	"github.com/sagernet/sing-dns"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
)

const WithQUIC = false

var ErrQUICNotIncluded = E.New(`QUIC is not included in this build, rebuild with -tags with_quic`)

func init() {
	dns.RegisterTransport([]string{"quic", "h3"}, func(ctx context.Context, dialer N.Dialer, link string) (dns.Transport, error) {
		return nil, ErrQUICNotIncluded
	})
}
