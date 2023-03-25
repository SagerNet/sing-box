package fakeip

import (
	"net/netip"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-dns"
	E "github.com/sagernet/sing/common/exceptions"
)

var _ adapter.FakeIPStore = (*Store)(nil)

type Store struct {
	router       adapter.Router
	inet4Range   netip.Prefix
	inet6Range   netip.Prefix
	storage      adapter.FakeIPStorage
	inet4Current netip.Addr
	inet6Current netip.Addr
}

func NewStore(router adapter.Router, inet4Range netip.Prefix, inet6Range netip.Prefix) *Store {
	return &Store{
		router:     router,
		inet4Range: inet4Range,
		inet6Range: inet6Range,
	}
}

func (s *Store) Start() error {
	var storage adapter.FakeIPStorage
	if clashServer := s.router.ClashServer(); clashServer != nil && clashServer.StoreFakeIP() {
		if cacheFile := clashServer.CacheFile(); cacheFile != nil {
			storage = cacheFile
		}
	}
	if storage == nil {
		storage = NewMemoryStorage()
	}
	metadata := storage.FakeIPMetadata()
	if metadata != nil && metadata.Inet4Range == s.inet4Range && metadata.Inet6Range == s.inet6Range {
		s.inet4Current = metadata.Inet4Current
		s.inet6Current = metadata.Inet6Current
	} else {
		if s.inet4Range.IsValid() {
			s.inet4Current = s.inet4Range.Addr().Next().Next()
		}
		if s.inet6Range.IsValid() {
			s.inet6Current = s.inet6Range.Addr().Next().Next()
		}
	}
	s.storage = storage
	return nil
}

func (s *Store) Contains(address netip.Addr) bool {
	return s.inet4Range.Contains(address) || s.inet6Range.Contains(address)
}

func (s *Store) Close() error {
	if s.storage == nil {
		return nil
	}
	return s.storage.FakeIPSaveMetadata(&adapter.FakeIPMetadata{
		Inet4Range:   s.inet4Range,
		Inet6Range:   s.inet6Range,
		Inet4Current: s.inet4Current,
		Inet6Current: s.inet6Current,
	})
}

func (s *Store) Create(domain string, strategy dns.DomainStrategy) (netip.Addr, error) {
	var address netip.Addr
	if strategy == dns.DomainStrategyUseIPv4 {
		if !s.inet4Current.IsValid() {
			return netip.Addr{}, E.New("missing IPv4 fakeip address range")
		}
		nextAddress := s.inet4Current.Next()
		if !s.inet4Range.Contains(nextAddress) {
			nextAddress = s.inet4Range.Addr().Next().Next()
		}
		s.inet4Current = nextAddress
		address = nextAddress
	} else {
		if !s.inet6Current.IsValid() {
			return netip.Addr{}, E.New("missing IPv6 fakeip address range")
		}
		nextAddress := s.inet6Current.Next()
		if !s.inet6Range.Contains(nextAddress) {
			nextAddress = s.inet6Range.Addr().Next().Next()
		}
		s.inet6Current = nextAddress
		address = nextAddress
	}
	err := s.storage.FakeIPStore(address, domain)
	if err != nil {
		return netip.Addr{}, err
	}
	return address, nil
}

func (s *Store) Lookup(address netip.Addr) (string, bool) {
	return s.storage.FakeIPLoad(address)
}

func (s *Store) Reset() error {
	return s.storage.FakeIPReset()
}
