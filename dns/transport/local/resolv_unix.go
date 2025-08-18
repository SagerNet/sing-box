//go:build !windows

package local

import (
	"bufio"
	"context"
	"net"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/miekg/dns"
)

func dnsReadConfig(_ context.Context, name string) *dnsConfig {
	conf := &dnsConfig{
		ndots:    1,
		timeout:  5 * time.Second,
		attempts: 2,
	}
	file, err := os.Open(name)
	if err != nil {
		conf.servers = defaultNS
		conf.search = dnsDefaultSearch()
		conf.err = err
		return conf
	}
	defer file.Close()
	fi, err := file.Stat()
	if err == nil {
		conf.mtime = fi.ModTime()
	} else {
		conf.servers = defaultNS
		conf.search = dnsDefaultSearch()
		conf.err = err
		return conf
	}
	reader := bufio.NewReader(file)
	var (
		prefix   []byte
		line     []byte
		isPrefix bool
	)
	for {
		line, isPrefix, err = reader.ReadLine()
		if err != nil {
			break
		}
		if isPrefix {
			prefix = append(prefix, line...)
			continue
		} else if len(prefix) > 0 {
			line = append(prefix, line...)
			prefix = nil
		}
		if len(line) > 0 && (line[0] == ';' || line[0] == '#') {
			continue
		}
		f := strings.Fields(string(line))
		if len(f) < 1 {
			continue
		}
		switch f[0] {
		case "nameserver":
			if len(f) > 1 && len(conf.servers) < 3 {
				if _, err := netip.ParseAddr(f[1]); err == nil {
					conf.servers = append(conf.servers, net.JoinHostPort(f[1], "53"))
				}
			}
		case "domain":
			if len(f) > 1 {
				conf.search = []string{dns.Fqdn(f[1])}
			}

		case "search":
			conf.search = make([]string, 0, len(f)-1)
			for i := 1; i < len(f); i++ {
				name := dns.Fqdn(f[i])
				if name == "." {
					continue
				}
				conf.search = append(conf.search, name)
			}

		case "options":
			for _, s := range f[1:] {
				switch {
				case strings.HasPrefix(s, "ndots:"):
					n, _, _ := dtoi(s[6:])
					if n < 0 {
						n = 0
					} else if n > 15 {
						n = 15
					}
					conf.ndots = n
				case strings.HasPrefix(s, "timeout:"):
					n, _, _ := dtoi(s[8:])
					if n < 1 {
						n = 1
					}
					conf.timeout = time.Duration(n) * time.Second
				case strings.HasPrefix(s, "attempts:"):
					n, _, _ := dtoi(s[9:])
					if n < 1 {
						n = 1
					}
					conf.attempts = n
				case s == "rotate":
					conf.rotate = true
				case s == "single-request" || s == "single-request-reopen":
					conf.singleRequest = true
				case s == "use-vc" || s == "usevc" || s == "tcp":
					conf.useTCP = true
				case s == "trust-ad":
					conf.trustAD = true
				case s == "edns0":
				case s == "no-reload":
					conf.noReload = true
				default:
					conf.unknownOpt = true
				}
			}

		case "lookup":
			conf.lookup = f[1:]

		default:
			conf.unknownOpt = true
		}
	}
	if len(conf.servers) == 0 {
		conf.servers = defaultNS
	}
	if len(conf.search) == 0 {
		conf.search = dnsDefaultSearch()
	}
	return conf
}

const big = 0xFFFFFF

func dtoi(s string) (n int, i int, ok bool) {
	n = 0
	for i = 0; i < len(s) && '0' <= s[i] && s[i] <= '9'; i++ {
		n = n*10 + int(s[i]-'0')
		if n >= big {
			return big, i, false
		}
	}
	if i == 0 {
		return 0, 0, false
	}
	return n, i, true
}
