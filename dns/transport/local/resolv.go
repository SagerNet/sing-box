package local

import (
	"context"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type resolverConfig struct {
	initOnce    sync.Once
	ch          chan struct{}
	lastChecked time.Time
	dnsConfig   atomic.Pointer[dnsConfig]
}

var resolvConf resolverConfig

func getSystemDNSConfig(ctx context.Context) *dnsConfig {
	resolvConf.tryUpdate(ctx, "/etc/resolv.conf")
	return resolvConf.dnsConfig.Load()
}

func (conf *resolverConfig) init(ctx context.Context) {
	conf.dnsConfig.Store(dnsReadConfig(ctx, "/etc/resolv.conf"))
	conf.lastChecked = time.Now()
	conf.ch = make(chan struct{}, 1)
}

func (conf *resolverConfig) tryUpdate(ctx context.Context, name string) {
	conf.initOnce.Do(func() {
		conf.init(ctx)
	})

	if conf.dnsConfig.Load().noReload {
		return
	}
	if !conf.tryAcquireSema() {
		return
	}
	defer conf.releaseSema()

	now := time.Now()
	if conf.lastChecked.After(now.Add(-5 * time.Second)) {
		return
	}
	conf.lastChecked = now
	if runtime.GOOS != "windows" {
		var mtime time.Time
		if fi, err := os.Stat(name); err == nil {
			mtime = fi.ModTime()
		}
		if mtime.Equal(conf.dnsConfig.Load().mtime) {
			return
		}
	}
	dnsConf := dnsReadConfig(ctx, name)
	conf.dnsConfig.Store(dnsConf)
}

func (conf *resolverConfig) tryAcquireSema() bool {
	select {
	case conf.ch <- struct{}{}:
		return true
	default:
		return false
	}
}

func (conf *resolverConfig) releaseSema() {
	<-conf.ch
}

type dnsConfig struct {
	servers       []string
	search        []string
	ndots         int
	timeout       time.Duration
	attempts      int
	rotate        bool
	unknownOpt    bool
	lookup        []string
	err           error
	mtime         time.Time
	soffset       uint32
	singleRequest bool
	useTCP        bool
	trustAD       bool
	noReload      bool
}

func (c *dnsConfig) serverOffset() uint32 {
	if c.rotate {
		return atomic.AddUint32(&c.soffset, 1) - 1 // return 0 to start
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
	// l++

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
	if name[len(name)-1] == '.' {
		name = name[:len(name)-1]
	}
	return strings.HasSuffix(name, ".onion")
}
