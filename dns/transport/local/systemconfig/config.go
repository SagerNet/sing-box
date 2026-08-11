package systemconfig

import (
	"net/netip"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	M "github.com/sagernet/sing/common/metadata"

	mDNS "github.com/miekg/dns"
)

var defaultServers = []M.Socksaddr{
	M.SocksaddrFrom(netip.AddrFrom4([4]byte{127, 0, 0, 1}), 53),
	M.SocksaddrFrom(netip.IPv6Loopback(), 53),
}

type Config struct {
	Servers       []M.Socksaddr
	Search        []string
	Ndots         int
	Timeout       time.Duration
	Attempts      int
	Rotate        bool
	soffset       uint32
	SingleRequest bool
	UseTCP        bool
	TrustAD       bool
}

func (c *Config) Equal(other *Config) bool {
	return slices.Equal(c.Servers, other.Servers) &&
		slices.Equal(c.Search, other.Search) &&
		c.Ndots == other.Ndots &&
		c.Timeout == other.Timeout &&
		c.Attempts == other.Attempts &&
		c.Rotate == other.Rotate &&
		c.SingleRequest == other.SingleRequest &&
		c.UseTCP == other.UseTCP &&
		c.TrustAD == other.TrustAD
}

func (c *Config) Signature() []string {
	signature := make([]string, 0, len(c.Servers)+len(c.Search)+1)
	for _, server := range c.Servers {
		signature = append(signature, server.String())
	}
	signature = append(signature, c.Search...)
	return append(signature, "ndots:"+strconv.Itoa(c.Ndots))
}

func (c *Config) ServerOffset() uint32 {
	if c.Rotate {
		return atomic.AddUint32(&c.soffset, 1) - 1
	}
	return 0
}

func (c *Config) NameList(name string) []string {
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

	hasNdots := strings.Count(name, ".") >= c.Ndots
	name += "."

	names := make([]string, 0, 1+len(c.Search))
	if hasNdots && !avoidDNS(name) {
		names = append(names, name)
	}
	for _, suffix := range c.Search {
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

func defaultSearch() []string {
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
