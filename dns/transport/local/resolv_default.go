package local

import (
	"os"
	"strings"
	_ "unsafe"

	"github.com/miekg/dns"
)

//go:linkname defaultNS net.defaultNS
var defaultNS []string

func dnsDefaultSearch() []string {
	hn, err := os.Hostname()
	if err != nil {
		return nil
	}
	if i := strings.IndexRune(hn, '.'); i >= 0 && i < len(hn)-1 {
		return []string{dns.Fqdn(hn[i+1:])}
	}
	return nil
}
