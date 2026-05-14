//go:build darwin && !cgo

package local

import (
	"context"

	E "github.com/sagernet/sing/common/exceptions"

	mDNS "github.com/miekg/dns"
)

func (t *Transport) systemExchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	return nil, E.New(`local DNS server requires CGO on darwin, rebuild with CGO_ENABLED=1`)
}
