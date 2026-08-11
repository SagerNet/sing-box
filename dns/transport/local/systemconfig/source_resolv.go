//go:build !windows && !(darwin && cgo)

package systemconfig

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

type Source struct {
	updateAccess sync.Mutex
	lastChecked  time.Time
	current      atomic.Pointer[resolvConfig]
}

type resolvConfig struct {
	config   *Config
	mtime    time.Time
	noReload bool
}

func NewSource(_ context.Context) *Source {
	source := &Source{lastChecked: time.Now()}
	source.current.Store(readResolvConfig(resolvConfPath))
	return source
}

func (s *Source) Configuration() *Config {
	s.tryUpdate()
	return s.current.Load().config
}

func (s *Source) tryUpdate() {
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
	updated := readResolvConfig(resolvConfPath)
	if updated.config.Equal(current.config) {
		updated.config = current.config
	}
	s.current.Store(updated)
}

func (s *Source) Reset() {
	s.updateAccess.Lock()
	s.lastChecked = time.Time{}
	s.updateAccess.Unlock()
}

func (s *Source) Close() error {
	return nil
}

func readResolvConfig(path string) *resolvConfig {
	config := &Config{
		Ndots:    1,
		Timeout:  5 * time.Second,
		Attempts: 2,
	}
	result := &resolvConfig{config: config}
	file, err := os.Open(path)
	if err != nil {
		config.Servers = defaultServers
		config.Search = defaultSearch()
		return result
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		config.Servers = defaultServers
		config.Search = defaultSearch()
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
			if len(fields) > 1 && len(config.Servers) < 3 {
				serverAddr, parseErr := netip.ParseAddr(fields[1])
				if parseErr == nil {
					config.Servers = append(config.Servers, M.SocksaddrFrom(serverAddr, 53))
				}
			}
		case "domain":
			if len(fields) > 1 {
				config.Search = []string{mDNS.Fqdn(fields[1])}
			}
		case "search":
			config.Search = make([]string, 0, len(fields)-1)
			for _, searchDomain := range fields[1:] {
				name := mDNS.Fqdn(searchDomain)
				if name == "." {
					continue
				}
				config.Search = append(config.Search, name)
			}
		case "options":
			for _, option := range fields[1:] {
				switch {
				case strings.HasPrefix(option, "ndots:"):
					value, parseErr := strconv.Atoi(option[len("ndots:"):])
					if parseErr == nil {
						config.Ndots = min(max(value, 0), 15)
					}
				case strings.HasPrefix(option, "timeout:"):
					value, parseErr := strconv.Atoi(option[len("timeout:"):])
					if parseErr == nil {
						config.Timeout = time.Duration(max(value, 1)) * time.Second
					}
				case strings.HasPrefix(option, "attempts:"):
					value, parseErr := strconv.Atoi(option[len("attempts:"):])
					if parseErr == nil {
						config.Attempts = max(value, 1)
					}
				case option == "rotate":
					config.Rotate = true
				case option == "single-request" || option == "single-request-reopen":
					config.SingleRequest = true
				case option == "use-vc" || option == "usevc" || option == "tcp":
					config.UseTCP = true
				case option == "trust-ad":
					config.TrustAD = true
				case option == "no-reload":
					result.noReload = true
				}
			}
		}
	}
	if len(config.Servers) == 0 {
		config.Servers = defaultServers
	}
	if len(config.Search) == 0 {
		config.Search = defaultSearch()
	}
	return result
}
