//go:build !darwin

package local

import (
	"context"
	"os"

	mDNS "github.com/miekg/dns"
)

func (t *Transport) systemExchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	return nil, os.ErrInvalid
}
