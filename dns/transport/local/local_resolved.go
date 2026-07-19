package local

import (
	"context"

	mDNS "github.com/miekg/dns"
)

type ResolvedResolver interface {
	Start() error
	Close() error
	Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error)
	ExchangeAsync(ctx context.Context, message *mDNS.Msg, callback func(response *mDNS.Msg, err error))
}
