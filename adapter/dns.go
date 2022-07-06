package adapter

import (
	"context"
	"net/netip"

	C "github.com/sagernet/sing-box/constant"

	"golang.org/x/net/dns/dnsmessage"
)

type DNSClient interface {
	Exchange(ctx context.Context, transport DNSTransport, message *dnsmessage.Message) (*dnsmessage.Message, error)
	Lookup(ctx context.Context, transport DNSTransport, domain string, strategy C.DomainStrategy) ([]netip.Addr, error)
}

type DNSTransport interface {
	Service
	Raw() bool
	Exchange(ctx context.Context, message *dnsmessage.Message) (*dnsmessage.Message, error)
	Lookup(ctx context.Context, domain string, strategy C.DomainStrategy) ([]netip.Addr, error)
}
