package local

import (
	"sync/atomic"
	"time"
	_ "unsafe"
)

const (
	// net.maxDNSPacketSize
	maxDNSPacketSize = 1232
)

type dnsConfig struct {
	servers       []string      // server addresses (in host:port form) to use
	search        []string      // rooted suffixes to append to local name
	ndots         int           // number of dots in name to trigger absolute lookup
	timeout       time.Duration // wait before giving up on a query, including retries
	attempts      int           // lost packets before giving up on server
	rotate        bool          // round robin among servers
	unknownOpt    bool          // anything unknown was encountered
	lookup        []string      // OpenBSD top-level database "lookup" order
	err           error         // any error that occurs during open of resolv.conf
	mtime         time.Time     // time of resolv.conf modification
	soffset       uint32        // used by serverOffset
	singleRequest bool          // use sequential A and AAAA queries instead of parallel queries
	useTCP        bool          // force usage of TCP for DNS resolutions
	trustAD       bool          // add AD flag to queries
	noReload      bool          // do not check for config file updates
}

func (c *dnsConfig) serverOffset() uint32 {
	if c.rotate {
		return atomic.AddUint32(&c.soffset, 1) - 1 // return 0 to start
	}
	return 0
}

//go:linkname runtime_rand runtime.rand
func runtime_rand() uint64

func randInt() int {
	return int(uint(runtime_rand()) >> 1) // clear sign bit
}
