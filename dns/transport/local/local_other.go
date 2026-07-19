//go:build !darwin

package local

import (
	"context"
	"os"

	mDNS "github.com/miekg/dns"
)

type systemResolver struct{}

func (r *systemResolver) close() {}

func (r *systemResolver) reset() {}

func (t *Transport) systemExchangeAsync(ctx context.Context, message *mDNS.Msg, callback func(response *mDNS.Msg, err error)) {
	callback(nil, os.ErrInvalid)
}
