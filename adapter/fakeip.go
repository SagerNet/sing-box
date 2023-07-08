package adapter

import (
	"net/netip"

	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing/common/logger"
)

type FakeIPStore interface {
	Service
	Contains(address netip.Addr) bool
	Create(domain string, strategy dns.DomainStrategy) (netip.Addr, error)
	Lookup(address netip.Addr) (string, bool)
	Reset() error
}

type FakeIPStorage interface {
	FakeIPMetadata() *FakeIPMetadata
	FakeIPSaveMetadata(metadata *FakeIPMetadata) error
	FakeIPStore(address netip.Addr, domain string) error
	FakeIPStoreAsync(address netip.Addr, domain string, logger logger.Logger)
	FakeIPLoad(address netip.Addr) (string, bool)
	FakeIPReset() error
}

type FakeIPTransport interface {
	dns.Transport
	Store() FakeIPStore
}
