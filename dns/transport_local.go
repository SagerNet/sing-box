package dns

import (
	"context"
	"net"
	"net/netip"
	"os"
	"sort"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"

	"golang.org/x/net/dns/dnsmessage"
)

var LocalTransportConstructor func() adapter.DNSTransport

func NewLocalTransport() adapter.DNSTransport {
	if LocalTransportConstructor != nil {
		return LocalTransportConstructor()
	}
	return &LocalTransport{}
}

var _ adapter.DNSTransport = (*LocalTransport)(nil)

type LocalTransport struct {
	resolver net.Resolver
}

func (t *LocalTransport) Start() error {
	return nil
}

func (t *LocalTransport) Close() error {
	return nil
}

func (t *LocalTransport) Raw() bool {
	return false
}

func (t *LocalTransport) Exchange(ctx context.Context, message *dnsmessage.Message) (*dnsmessage.Message, error) {
	return nil, os.ErrInvalid
}

func (t *LocalTransport) Lookup(ctx context.Context, domain string, strategy C.DomainStrategy) ([]netip.Addr, error) {
	var network string
	switch strategy {
	case C.DomainStrategyAsIS, C.DomainStrategyPreferIPv4, C.DomainStrategyPreferIPv6:
		network = "ip"
	case C.DomainStrategyUseIPv4:
		network = "ip4"
	case C.DomainStrategyUseIPv6:
		network = "ip6"
	}
	addrs, err := t.resolver.LookupNetIP(ctx, network, domain)
	if err != nil {
		return nil, err
	}
	addrs = common.Map(addrs, func(it netip.Addr) netip.Addr {
		if it.Is4In6() {
			return netip.AddrFrom4(it.As4())
		}
		return it
	})
	switch strategy {
	case C.DomainStrategyPreferIPv4:
		sort.Slice(addrs, func(i, j int) bool {
			return addrs[i].Is4() && addrs[j].Is6()
		})
	case C.DomainStrategyPreferIPv6:
		sort.Slice(addrs, func(i, j int) bool {
			return addrs[i].Is6() && addrs[j].Is4()
		})
	}
	return addrs, nil
}
