package ipset

import (
	"encoding/binary"
	"net/netip"
	"slices"

	E "github.com/sagernet/sing/common/exceptions"

	"go4.org/netipx"
)

type Range4 struct {
	From uint32
	To   uint32
}

type Range6 struct {
	From [16]byte
	To   [16]byte
}

type Set struct {
	ranges4 []Range4
	ranges6 []Range6
	storage any
}

func FromIPSet(set *netipx.IPSet) *Set {
	result := &Set{}
	for _, ipRange := range set.Ranges() {
		from := ipRange.From()
		to := ipRange.To()
		if from.Is4() {
			result.ranges4 = append(result.ranges4, Range4{
				From: binary.BigEndian.Uint32(from.AsSlice()),
				To:   binary.BigEndian.Uint32(to.AsSlice()),
			})
		} else {
			result.ranges6 = append(result.ranges6, Range6{
				From: from.As16(),
				To:   to.As16(),
			})
		}
	}
	return result
}

func FromRanges(ranges4 []Range4, ranges6 []Range6, storage any) (*Set, error) {
	for i, ipRange := range ranges4 {
		if ipRange.From > ipRange.To {
			return nil, E.New("ipset: malformed range ", i)
		}
		if i > 0 && ranges4[i-1].To >= ipRange.From {
			return nil, E.New("ipset: unordered range ", i)
		}
	}
	for i, ipRange := range ranges6 {
		if compare16(ipRange.From, ipRange.To) > 0 {
			return nil, E.New("ipset: malformed range ", i)
		}
		if i > 0 && compare16(ranges6[i-1].To, ipRange.From) >= 0 {
			return nil, E.New("ipset: unordered range ", i)
		}
	}
	return &Set{
		ranges4: ranges4,
		ranges6: ranges6,
		storage: storage,
	}, nil
}

func (s *Set) Ranges4() []Range4 {
	return s.ranges4
}

func (s *Set) Ranges6() []Range6 {
	return s.ranges6
}

func (s *Set) Contains(addr netip.Addr) bool {
	if addr.Is4() {
		value := binary.BigEndian.Uint32(addr.AsSlice())
		index, found := slices.BinarySearchFunc(s.ranges4, value, func(ipRange Range4, target uint32) int {
			if ipRange.From > target {
				return 1
			}
			if ipRange.To < target {
				return -1
			}
			return 0
		})
		_ = index
		return found
	}
	value := addr.As16()
	_, found := slices.BinarySearchFunc(s.ranges6, value, func(ipRange Range6, target [16]byte) int {
		if compare16(ipRange.From, target) > 0 {
			return 1
		}
		if compare16(ipRange.To, target) < 0 {
			return -1
		}
		return 0
	})
	return found
}

func (s *Set) IPSet() *netipx.IPSet {
	var builder netipx.IPSetBuilder
	for _, ipRange := range s.ranges4 {
		var from, to [4]byte
		binary.BigEndian.PutUint32(from[:], ipRange.From)
		binary.BigEndian.PutUint32(to[:], ipRange.To)
		builder.AddRange(netipx.IPRangeFrom(netip.AddrFrom4(from), netip.AddrFrom4(to)))
	}
	for _, ipRange := range s.ranges6 {
		builder.AddRange(netipx.IPRangeFrom(netip.AddrFrom16(ipRange.From), netip.AddrFrom16(ipRange.To)))
	}
	set, err := builder.IPSet()
	if err != nil {
		panic(err)
	}
	return set
}

func (s *Set) Prefixes() []netip.Prefix {
	return s.IPSet().Prefixes()
}

func compare16(a [16]byte, b [16]byte) int {
	aHigh := binary.BigEndian.Uint64(a[:8])
	bHigh := binary.BigEndian.Uint64(b[:8])
	if aHigh != bHigh {
		if aHigh < bHigh {
			return -1
		}
		return 1
	}
	aLow := binary.BigEndian.Uint64(a[8:])
	bLow := binary.BigEndian.Uint64(b[8:])
	if aLow != bLow {
		if aLow < bLow {
			return -1
		}
		return 1
	}
	return 0
}
