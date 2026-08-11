//go:build !windows && !(darwin && cgo)

package local

import (
	"bufio"
	"context"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	M "github.com/sagernet/sing/common/metadata"

	mDNS "github.com/miekg/dns"
)

const resolvConfPath = "/etc/resolv.conf"

type systemConfigSource struct {
	updateAccess sync.Mutex
	lastChecked  time.Time
	current      atomic.Pointer[resolvConfig]
}

type resolvConfig struct {
	config   *dnsConfig
	mtime    time.Time
	noReload bool
}

func newSystemConfigSource(_ context.Context) *systemConfigSource {
	source := &systemConfigSource{lastChecked: time.Now()}
	source.current.Store(dnsReadConfig(resolvConfPath))
	return source
}

func (s *systemConfigSource) Configuration() *dnsConfig {
	s.tryUpdate()
	return s.current.Load().config
}

func (s *systemConfigSource) tryUpdate() {
	if s.current.Load().noReload {
		return
	}
	if !s.updateAccess.TryLock() {
		return
	}
	defer s.updateAccess.Unlock()
	now := time.Now()
	if s.lastChecked.After(now.Add(-5 * time.Second)) {
		return
	}
	s.lastChecked = now
	var mtime time.Time
	fileInfo, err := os.Stat(resolvConfPath)
	if err == nil {
		mtime = fileInfo.ModTime()
	}
	current := s.current.Load()
	if mtime.Equal(current.mtime) {
		return
	}
	updated := dnsReadConfig(resolvConfPath)
	if updated.config.equal(current.config) {
		updated.config = current.config
	}
	s.current.Store(updated)
}

func (s *systemConfigSource) Reset() {
	s.updateAccess.Lock()
	s.lastChecked = time.Time{}
	s.updateAccess.Unlock()
}

func (s *systemConfigSource) Close() error {
	return nil
}

func dnsReadConfig(path string) *resolvConfig {
	config := &dnsConfig{
		ndots:    1,
		timeout:  5 * time.Second,
		attempts: 2,
	}
	result := &resolvConfig{config: config}
	file, err := os.Open(path)
	if err != nil {
		config.servers = defaultNS
		config.search = dnsDefaultSearch()
		return result
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		config.servers = defaultNS
		config.search = dnsDefaultSearch()
		return result
	}
	result.mtime = fileInfo.ModTime()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}
		switch fields[0] {
		case "nameserver":
			if len(fields) > 1 && len(config.servers) < 3 {
				serverAddr, parseErr := netip.ParseAddr(fields[1])
				if parseErr == nil {
					config.servers = append(config.servers, M.SocksaddrFrom(serverAddr, 53))
				}
			}
		case "domain":
			if len(fields) > 1 {
				config.search = []string{mDNS.Fqdn(fields[1])}
			}
		case "search":
			config.search = make([]string, 0, len(fields)-1)
			for _, searchDomain := range fields[1:] {
				name := mDNS.Fqdn(searchDomain)
				if name == "." {
					continue
				}
				config.search = append(config.search, name)
			}
		case "options":
			for _, option := range fields[1:] {
				switch {
				case strings.HasPrefix(option, "ndots:"):
					value, parseErr := strconv.Atoi(option[len("ndots:"):])
					if parseErr == nil {
						config.ndots = min(max(value, 0), 15)
					}
				case strings.HasPrefix(option, "timeout:"):
					value, parseErr := strconv.Atoi(option[len("timeout:"):])
					if parseErr == nil {
						config.timeout = time.Duration(max(value, 1)) * time.Second
					}
				case strings.HasPrefix(option, "attempts:"):
					value, parseErr := strconv.Atoi(option[len("attempts:"):])
					if parseErr == nil {
						config.attempts = max(value, 1)
					}
				case option == "rotate":
					config.rotate = true
				case option == "single-request" || option == "single-request-reopen":
					config.singleRequest = true
				case option == "use-vc" || option == "usevc" || option == "tcp":
					config.useTCP = true
				case option == "trust-ad":
					config.trustAD = true
				case option == "no-reload":
					result.noReload = true
				}
			}
		}
	}
	if len(config.servers) == 0 {
		config.servers = defaultNS
	}
	if len(config.search) == 0 {
		config.search = dnsDefaultSearch()
	}
	return result
}
