package local

import (
	"context"
	"net/netip"

	mDNS "github.com/miekg/dns"
)

type ResolvedResolver interface {
	Start() error
	Close() error
	Reset()
	Environment() []string
	ServerAddresses() []netip.Addr
	ExchangeAsync(ctx context.Context, message *mDNS.Msg, callback func(response *mDNS.Msg, err error))
}
