package kernelports

import (
	"net/netip"
	"slices"
	"sync/atomic"

	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing-tun/gtcpip/header"
	E "github.com/sagernet/sing/common/exceptions"
)

var ErrExhausted = E.New("kernel ports: no more ports can be reserved")

func (p *Pool) ranges(protocol uint8) *atomic.Pointer[[]tun.SelectorRange] {
	if protocol == uint8(header.UDPProtocolNumber) {
		return &p.udp
	}
	return &p.tcp
}

func (p *Pool) PortSelectorRanges(protocol uint8) []tun.SelectorRange {
	ranges := p.ranges(protocol).Load()
	if ranges == nil {
		return nil
	}
	return *ranges
}

func (p *Pool) ExpandSelectorRanges(protocol uint8) bool {
	_, err := p.Acquire(protocol)
	return err == nil
}

func (p *Pool) Contains(protocol uint8, port uint16) bool {
	return slices.ContainsFunc(p.PortSelectorRanges(protocol), func(selectorRange tun.SelectorRange) bool {
		return port >= selectorRange.Start && port-selectorRange.Start < selectorRange.Count
	})
}

func (p *Pool) ReleaseSelector(protocol uint8, address netip.AddrPort) {
}

var (
	_ tun.PortWithSelectorRange       = (*Pool)(nil)
	_ tun.PortWithSelectorReservation = (*Pool)(nil)
)
