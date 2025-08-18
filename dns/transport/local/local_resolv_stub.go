//go:build !darwin

package local

import (
	"context"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
)

func NewResolvTransport(ctx context.Context, logger log.ContextLogger, tag string) (adapter.DNSTransport, error) {
	return nil, os.ErrInvalid
}
