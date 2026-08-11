package local

import (
	"net/netip"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	M "github.com/sagernet/sing/common/metadata"

	mDNS "github.com/miekg/dns"
)

var defaultNS = []M.Socksaddr{
	M.SocksaddrFrom(netip.AddrFrom4([4]byte{127, 0, 0, 1}), 53),
	M.SocksaddrFrom(netip.IPv6Loopback(), 53),
}

type dnsConfig struct {
	servers       []M.Socksaddr
	search        []string
	ndots         int
	timeout       time.Duration
	attempts      int
	rotate        bool
	soffset       uint32
	singleRequest bool
	useTCP        bool
	trustAD       bool
}

func (c *dnsConfig) equal(other *dnsConfig) bool {
	return slices.Equal(c.servers, other.servers) &&
		slices.Equal(c.search, other.search) &&
		c.ndots == other.ndots &&
		c.timeout == other.timeout &&
		c.attempts == other.attempts &&
		c.rotate == other.rotate &&
		c.singleRequest == other.singleRequest &&
		c.useTCP == other.useTCP &&
		c.trustAD == other.trustAD
}

func (c *dnsConfig) serverOffset() uint32 {
	if c.rotate {
		return atomic.AddUint32(&c.soffset, 1) - 1
	}
	return 0
}

func (c *dnsConfig) nameList(name string) []string {
	l := len(name)
	rooted := l > 0 && name[l-1] == '.'
	if l > 254 || l == 254 && !rooted {
		return nil
	}

	if rooted {
		if avoidDNS(name) {
			return nil
		}
		return []string{name}
	}

	hasNdots := strings.Count(name, ".") >= c.ndots
	name += "."

	names := make([]string, 0, 1+len(c.search))
	if hasNdots && !avoidDNS(name) {
		names = append(names, name)
	}
	for _, suffix := range c.search {
		fqdn := name + suffix
		if !avoidDNS(fqdn) && len(fqdn) <= 254 {
			names = append(names, fqdn)
		}
	}
	if !hasNdots && !avoidDNS(name) {
		names = append(names, name)
	}
	return names
}

func avoidDNS(name string) bool {
	if name == "" {
		return true
	}
	return strings.HasSuffix(strings.TrimSuffix(name, "."), ".onion")
}

func dnsDefaultSearch() []string {
	hostname, err := os.Hostname()
	if err != nil {
		return nil
	}
	_, domain, found := strings.Cut(hostname, ".")
	if !found || domain == "" {
		return nil
	}
	return []string{mDNS.Fqdn(domain)}
}
