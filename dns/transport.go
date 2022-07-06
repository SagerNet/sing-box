package dns

import (
	"context"
	"net/netip"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"

	"golang.org/x/net/dns/dnsmessage"
)

type Transport interface {
	adapter.Service
	Raw() bool
	Exchange(ctx context.Context, message *dnsmessage.Message) (*dnsmessage.Message, error)
	Lookup(ctx context.Context, domain string, strategy C.DomainStrategy) ([]netip.Addr, error)
}
