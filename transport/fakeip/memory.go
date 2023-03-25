package fakeip

import (
	"net/netip"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/cache"
)

var _ adapter.FakeIPStorage = (*MemoryStorage)(nil)

type MemoryStorage struct {
	metadata    *adapter.FakeIPMetadata
	domainCache *cache.LruCache[netip.Addr, string]
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		domainCache: cache.New[netip.Addr, string](),
	}
}

func (s *MemoryStorage) FakeIPMetadata() *adapter.FakeIPMetadata {
	return s.metadata
}

func (s *MemoryStorage) FakeIPSaveMetadata(metadata *adapter.FakeIPMetadata) error {
	s.metadata = metadata
	return nil
}

func (s *MemoryStorage) FakeIPStore(address netip.Addr, domain string) error {
	s.domainCache.Store(address, domain)
	return nil
}

func (s *MemoryStorage) FakeIPLoad(address netip.Addr) (string, bool) {
	return s.domainCache.Load(address)
}

func (s *MemoryStorage) FakeIPReset() error {
	s.domainCache = cache.New[netip.Addr, string]()
	return nil
}
