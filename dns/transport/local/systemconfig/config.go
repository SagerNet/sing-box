package systemconfig

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

var defaultServers = []M.Socksaddr{
	M.SocksaddrFrom(netip.AddrFrom4([4]byte{127, 0, 0, 1}), 53),
	M.SocksaddrFrom(netip.IPv6Loopback(), 53),
}

type Config struct {
	Servers         []M.Socksaddr
	Search          []string
	Timeout         time.Duration
	Attempts        int
	Rotate          bool
	soffset         uint32
	SingleRequest   bool
	UseTCP          bool
	TrustAD         bool
	SystemdResolved bool
}

func (c *Config) Equal(other *Config) bool {
	return slices.Equal(c.Servers, other.Servers) &&
		slices.Equal(c.Search, other.Search) &&
		c.Timeout == other.Timeout &&
		c.Attempts == other.Attempts &&
		c.Rotate == other.Rotate &&
		c.SingleRequest == other.SingleRequest &&
		c.UseTCP == other.UseTCP &&
		c.TrustAD == other.TrustAD &&
		c.SystemdResolved == other.SystemdResolved
}

func (c *Config) Signature() []string {
	signature := make([]string, 0, len(c.Servers)+len(c.Search))
	for _, server := range c.Servers {
		signature = append(signature, server.String())
	}
	return append(signature, c.Search...)
}

func (c *Config) ServerOffset() uint32 {
	if c.Rotate {
		return atomic.AddUint32(&c.soffset, 1) - 1
	}
	return 0
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
