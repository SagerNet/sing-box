package adapter

import (
	"net/netip"

	"github.com/sagernet/sing-dns"
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
	FakeIPLoad(address netip.Addr) (string, bool)
	FakeIPReset() error
}
