package local

import (
	"context"

	mDNS "github.com/miekg/dns"
)

type ResolvedResolver interface {
	Start() error
	Close() error
	Object() any
	Exchange(object any, ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error)
}
