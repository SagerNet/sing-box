package fakeip

import (
	"context"
	"net/netip"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/service"
)

const reservedAddressCount = 1024

var _ adapter.FakeIPStore = (*Store)(nil)

type Store struct {
	ctx     context.Context
	logger  logger.Logger
	storage adapter.FakeIPStorage

	addressAccess sync.Mutex
	inet4         addressPool
	inet6         addressPool
}

type addressPool struct {
	addressRange  netip.Prefix
	last          netip.Addr
	reservedStep  int
	current       netip.Addr
	reservedEnd   netip.Addr
	reservedCount int
}

func NewStore(ctx context.Context, logger logger.Logger, inet4Range netip.Prefix, inet6Range netip.Prefix) (*Store, error) {
	if inet4Range.IsValid() && inet4Range.Addr().BitLen()-inet4Range.Bits() < 2 {
		return nil, E.New("inet4_range must have at least 2 host bits")
	}
	if inet6Range.IsValid() && inet6Range.Addr().BitLen()-inet6Range.Bits() < 2 {
		return nil, E.New("inet6_range must have at least 2 host bits")
	}
	return &Store{
		ctx:    ctx,
		logger: logger,
		inet4:  newAddressPool(inet4Range),
		inet6:  newAddressPool(inet6Range),
	}, nil
}

func newAddressPool(addressRange netip.Prefix) addressPool {
	pool := addressPool{
		addressRange: addressRange,
	}
	if addressRange.IsValid() {
		pool.last = broadcastAddress(addressRange)
		hostBits := addressRange.Addr().BitLen() - addressRange.Bits()
		if hostBits > 11 {
			pool.reservedStep = reservedAddressCount
		} else {
			addressCount := 1 << hostBits
			pool.reservedStep = max((addressCount-3)/2, 1)
		}
	}
	return pool
}

func broadcastAddress(prefix netip.Prefix) netip.Addr {
	addr := prefix.Addr()
	raw := addr.As16()
	bits := prefix.Bits()
	if addr.Is4() {
		bits += 96
	}
	for i := bits; i < 128; i++ {
		raw[i/8] |= 1 << (7 - i%8)
	}
	if addr.Is4() {
		return netip.AddrFrom4([4]byte(raw[12:]))
	}
	return netip.AddrFrom16(raw)
}

func (p *addressPool) next(current netip.Addr) netip.Addr {
	address := current.Next()
	if address == p.last || !p.addressRange.Contains(address) {
		address = p.addressRange.Addr().Next().Next()
	}
	return address
}

func (p *addressPool) resetReservation() {
	p.reservedCount = 0
	p.reservedEnd = p.current
}

func (s *Store) Start() error {
	var storage adapter.FakeIPStorage
	cacheFile := service.FromContext[adapter.CacheFile](s.ctx)
	if cacheFile != nil && cacheFile.StoreFakeIP() {
		storage = cacheFile
	}
	if storage == nil {
		storage = NewMemoryStorage()
	}
	metadata := storage.FakeIPMetadata()
	if metadata != nil && metadata.Inet4Range == s.inet4.addressRange && metadata.Inet6Range == s.inet6.addressRange {
		s.inet4.current = metadata.Inet4Current
		s.inet6.current = metadata.Inet6Current
	} else {
		if s.inet4.addressRange.IsValid() {
			s.inet4.current = s.inet4.addressRange.Addr().Next()
		}
		if s.inet6.addressRange.IsValid() {
			s.inet6.current = s.inet6.addressRange.Addr().Next()
		}
		_ = storage.FakeIPReset()
	}
	s.inet4.reservedEnd = s.inet4.current
	s.inet6.reservedEnd = s.inet6.current
	s.storage = storage
	return nil
}

func (s *Store) Contains(address netip.Addr) bool {
	return s.inet4.addressRange.Contains(address) || s.inet6.addressRange.Contains(address)
}

func (s *Store) Close() error {
	if s.storage == nil {
		return nil
	}
	s.addressAccess.Lock()
	defer s.addressAccess.Unlock()
	s.inet4.resetReservation()
	s.inet6.resetReservation()
	return s.storage.FakeIPSaveMetadata(&adapter.FakeIPMetadata{
		Inet4Range:   s.inet4.addressRange,
		Inet6Range:   s.inet6.addressRange,
		Inet4Current: s.inet4.current,
		Inet6Current: s.inet6.current,
	})
}

func (s *Store) Create(domain string, isIPv6 bool) (netip.Addr, error) {
	if address, loaded := s.storage.FakeIPLoadDomain(domain, isIPv6); loaded {
		return address, nil
	}

	s.addressAccess.Lock()
	defer s.addressAccess.Unlock()

	// Double-check after acquiring lock
	if address, loaded := s.storage.FakeIPLoadDomain(domain, isIPv6); loaded {
		return address, nil
	}

	pool := &s.inet4
	if isIPv6 {
		pool = &s.inet6
	}
	if !pool.current.IsValid() {
		if isIPv6 {
			return netip.Addr{}, E.New("missing IPv6 fakeip address range")
		}
		return netip.Addr{}, E.New("missing IPv4 fakeip address range")
	}
	if pool.reservedCount == 0 {
		reservedEnd := pool.current
		for range pool.reservedStep {
			reservedEnd = pool.next(reservedEnd)
		}
		metadata := &adapter.FakeIPMetadata{
			Inet4Range:   s.inet4.addressRange,
			Inet6Range:   s.inet6.addressRange,
			Inet4Current: s.inet4.reservedEnd,
			Inet6Current: s.inet6.reservedEnd,
		}
		if isIPv6 {
			metadata.Inet6Current = reservedEnd
		} else {
			metadata.Inet4Current = reservedEnd
		}
		err := s.storage.FakeIPReserve(metadata, pool.current, reservedEnd)
		if err != nil {
			return netip.Addr{}, E.Cause(err, "save fakeip metadata")
		}
		pool.reservedEnd = reservedEnd
		pool.reservedCount = pool.reservedStep
	}
	pool.reservedCount--
	pool.current = pool.next(pool.current)
	err := s.storage.FakeIPStore(pool.current, domain)
	if err != nil {
		s.logger.Warn("save FakeIP cache: ", err)
	}
	return pool.current, nil
}

func (s *Store) Lookup(address netip.Addr) (string, bool) {
	return s.storage.FakeIPLoad(address)
}

func (s *Store) Reset() error {
	s.addressAccess.Lock()
	defer s.addressAccess.Unlock()
	s.inet4.resetReservation()
	s.inet6.resetReservation()
	return s.storage.FakeIPReset()
}
