package iterator

import (
	"crypto/rand"
	"errors"
	"math/big"
	"net"
	"net/netip"

	"github.com/sagernet/sing-box/ipscanner/internal/statute"
)

// LCG represents a linear congruential generator with full period.
type LCG struct {
	modulus    *big.Int
	multiplier *big.Int
	increment  *big.Int
	current    *big.Int
}

// NewLCG creates a new LCG instance with a given size.
func NewLCG(size *big.Int) *LCG {
	modulus := new(big.Int).Set(size)

	// Generate random multiplier (a) and increment (c) that satisfy Hull-Dobell Theorem
	var multiplier, increment *big.Int
	for {
		var err error
		multiplier, err = rand.Int(rand.Reader, modulus)
		if err != nil {
			continue
		}
		increment, err = rand.Int(rand.Reader, modulus)
		if err != nil {
			continue
		}

		// Check Hull-Dobell Theorem conditions
		if checkHullDobell(modulus, multiplier, increment) {
			break
		}
	}

	return &LCG{
		modulus:    modulus,
		multiplier: multiplier,
		increment:  increment,
		current:    big.NewInt(0),
	}
}

// checkHullDobell checks if the given parameters satisfy the Hull-Dobell Theorem.
func checkHullDobell(modulus, multiplier, increment *big.Int) bool {
	// c and m are relatively prime
	gcd := new(big.Int).GCD(nil, nil, increment, modulus)
	if gcd.Cmp(big.NewInt(1)) != 0 {
		return false
	}

	// a - 1 is divisible by all prime factors of m
	aMinusOne := new(big.Int).Sub(multiplier, big.NewInt(1))

	// a - 1 is divisible by 4 if m is divisible by 4
	if new(big.Int).And(modulus, big.NewInt(3)).Cmp(big.NewInt(0)) == 0 {
		if new(big.Int).And(aMinusOne, big.NewInt(3)).Cmp(big.NewInt(0)) != 0 {
			return false
		}
	}

	return true
}

// Next generates the next number in the sequence.
func (lcg *LCG) Next() *big.Int {
	if lcg.current.Cmp(lcg.modulus) == 0 {
		return nil // Sequence complete
	}

	next := new(big.Int)
	next.Mul(lcg.multiplier, lcg.current)
	next.Add(next, lcg.increment)
	next.Mod(next, lcg.modulus)

	lcg.current.Set(next)
	return next
}

type ipRange struct {
	lcg   *LCG
	start netip.Addr
	stop  netip.Addr
	size  *big.Int
	index *big.Int
}

func newIPRange(cidr netip.Prefix) (ipRange, error) {
	startIP := cidr.Addr()
	stopIP := lastIP(cidr)
	size := ipRangeSize(cidr)
	return ipRange{
		start: startIP,
		stop:  stopIP,
		size:  size,
		index: big.NewInt(0),
		lcg:   NewLCG(size),
	}, nil
}

func lastIP(prefix netip.Prefix) netip.Addr {
	// Calculate the number of bits to fill for the last address based on the address family
	fillBits := 128 - prefix.Bits()
	if prefix.Addr().Is4() {
		fillBits = 32 - prefix.Bits()
	}

	// Calculate the numerical representation of the last address by setting the remaining bits to 1
	var lastAddrInt big.Int
	lastAddrInt.SetBytes(prefix.Addr().AsSlice())
	for i := 0; i < fillBits; i++ {
		lastAddrInt.SetBit(&lastAddrInt, i, 1)
	}

	// Convert the big.Int back to netip.Addr
	lastAddrBytes := lastAddrInt.Bytes()
	var lastAddr netip.Addr
	if prefix.Addr().Is4() {
		// Ensure the slice is the right length for IPv4
		if len(lastAddrBytes) < net.IPv4len {
			leadingZeros := make([]byte, net.IPv4len-len(lastAddrBytes))
			lastAddrBytes = append(leadingZeros, lastAddrBytes...)
		}
		lastAddr, _ = netip.AddrFromSlice(lastAddrBytes[len(lastAddrBytes)-net.IPv4len:])
	} else {
		// Ensure the slice is the right length for IPv6
		if len(lastAddrBytes) < net.IPv6len {
			leadingZeros := make([]byte, net.IPv6len-len(lastAddrBytes))
			lastAddrBytes = append(leadingZeros, lastAddrBytes...)
		}
		lastAddr, _ = netip.AddrFromSlice(lastAddrBytes)
	}

	return lastAddr
}

func addIP(ip netip.Addr, num *big.Int) netip.Addr {
	addrAs16 := ip.As16()
	ipInt := new(big.Int).SetBytes(addrAs16[:])
	ipInt.Add(ipInt, num)
	addr, _ := netip.AddrFromSlice(ipInt.FillBytes(make([]byte, 16)))
	return addr.Unmap()
}

func ipRangeSize(prefix netip.Prefix) *big.Int {
	// The number of bits in the address depends on whether it's IPv4 or IPv6.
	totalBits := 128 // Assume IPv6 by default
	if prefix.Addr().Is4() {
		totalBits = 32 // Adjust for IPv4
	}

	// Calculate the size of the range
	bits := prefix.Bits() // This is the prefix length
	size := big.NewInt(1)
	size.Lsh(size, uint(totalBits-bits)) // Left shift to calculate the range size

	return size
}

type IpGenerator struct {
	ipRanges []ipRange
}

func (g *IpGenerator) NextBatch() ([]netip.Addr, error) {
	var results []netip.Addr
	for i, r := range g.ipRanges {
		if r.index.Cmp(r.size) >= 0 {
			continue
		}
		shuffleIndex := r.lcg.Next()
		if shuffleIndex == nil {
			continue
		}
		results = append(results, addIP(r.start, shuffleIndex))
		g.ipRanges[i].index.Add(g.ipRanges[i].index, big.NewInt(1))
	}
	if len(results) == 0 {
		okFlag := false
		for i := range g.ipRanges {
			if g.ipRanges[i].index.Cmp(big.NewInt(0)) > 0 {
				okFlag = true
			}
			g.ipRanges[i].index.SetInt64(0)
		}
		if okFlag {
			// Reshuffle and start over
			for i := range g.ipRanges {
				g.ipRanges[i].lcg = NewLCG(g.ipRanges[i].size)
			}
			return g.NextBatch()
		} else {
			return nil, errors.New("no more IP addresses")
		}
	}
	return results, nil
}

// shuffleSubnetsIpRange shuffles a slice of ipRange using crypto/rand
func shuffleSubnetsIpRange(subnets []ipRange) error {
	for i := range subnets {
		jBig, err := rand.Int(rand.Reader, big.NewInt(int64(len(subnets))))
		if err != nil {
			return err
		}
		j := jBig.Int64()

		subnets[i], subnets[j] = subnets[j], subnets[i]
	}
	return nil
}

func NewIterator(opts *statute.ScannerOptions) *IpGenerator {
	var ranges []ipRange
	for _, cidr := range opts.CidrList {
		if !opts.UseIPv6 && cidr.Addr().Is6() {
			continue
		}
		if !opts.UseIPv4 && cidr.Addr().Is4() {
			continue
		}

		ipRange, err := newIPRange(cidr)
		if err != nil {
			// TODO
			continue
		}
		ranges = append(ranges, ipRange)
	}
	if len(ranges) == 0 {
		// TODO
		return nil
	}
	err := shuffleSubnetsIpRange(ranges)
	if err != nil {
		// TODO
		return nil
	}
	return &IpGenerator{ipRanges: ranges}
}
