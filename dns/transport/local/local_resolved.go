package local

import (
	"context"

	mDNS "github.com/miekg/dns"
)

type ResolvedResolver interface {
	Start() error
	Close() error
	Available() bool
	Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error)
}
