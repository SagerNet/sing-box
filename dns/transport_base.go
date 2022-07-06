package dns

import (
	"context"
	"net/netip"
	"os"
	"sync"

	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
)

type myTransportAdapter struct {
	ctx         context.Context
	dialer      N.Dialer
	logger      log.Logger
	destination M.Socksaddr
	done        chan struct{}
	access      sync.RWMutex
	connection  *dnsConnection
}

func (t *myTransportAdapter) Start() error {
	return nil
}

func (t *myTransportAdapter) Close() error {
	select {
	case <-t.done:
		return os.ErrClosed
	default:
	}
	close(t.done)
	return nil
}

func (t *myTransportAdapter) Raw() bool {
	return true
}

func (t *myTransportAdapter) Lookup(ctx context.Context, domain string, strategy C.DomainStrategy) ([]netip.Addr, error) {
	return nil, os.ErrInvalid
}
