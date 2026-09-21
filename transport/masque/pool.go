package masque

import (
	"net/netip"
	"sync"

	"go4.org/netipx"
)

type addressPool struct {
	access    sync.Mutex
	prefix    netip.Prefix
	reserved  netip.Addr
	last      netip.Addr
	next      netip.Addr
	allocated map[netip.Addr]struct{}
}

func newAddressPool(address netip.Prefix) *addressPool {
	prefix := address.Masked()
	return &addressPool{
		prefix:    prefix,
		reserved:  address.Addr(),
		last:      netipx.PrefixLastIP(prefix),
		next:      prefix.Addr(),
		allocated: make(map[netip.Addr]struct{}),
	}
}

func (p *addressPool) usable(address netip.Addr) bool {
	if address == p.reserved || address == p.prefix.Addr() {
		return false
	}
	return !address.Is4() || address != p.last
}

func (p *addressPool) allocate() (netip.Addr, bool) {
	p.access.Lock()
	defer p.access.Unlock()
	start := p.next
	for {
		candidate := p.next
		if candidate == p.last {
			p.next = p.prefix.Addr()
		} else {
			p.next = candidate.Next()
		}
		_, inUse := p.allocated[candidate]
		if !inUse && p.usable(candidate) {
			p.allocated[candidate] = struct{}{}
			return candidate, true
		}
		if p.next == start {
			return netip.Addr{}, false
		}
	}
}

func (p *addressPool) release(address netip.Addr) {
	p.access.Lock()
	defer p.access.Unlock()
	delete(p.allocated, address)
}
