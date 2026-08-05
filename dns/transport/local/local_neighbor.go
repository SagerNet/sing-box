package local

import (
	"strings"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	E "github.com/sagernet/sing/common/exceptions"

	mDNS "github.com/miekg/dns"
)

func buildNeighborMatchers(domains []string) ([]string, error) {
	if len(domains) == 0 {
		return nil, nil
	}
	var suffixes []string
	for _, domain := range domains {
		if !strings.HasPrefix(domain, ".") {
			return nil, E.New("neighbor_domain entry must start with '.': ", domain)
		}
		suffixes = append(suffixes, mDNS.CanonicalName(domain))
	}
	return suffixes, nil
}

func (r *PreferredDomainResolver) lookupNeighbor(message *mDNS.Msg) *mDNS.Msg {
	if r.neighborResolver == nil {
		return nil
	}
	question := message.Question[0]
	if question.Qtype != mDNS.TypeA && question.Qtype != mDNS.TypeAAAA {
		return nil
	}
	host := extractNeighborHost(mDNS.CanonicalName(question.Name), r.neighborSuffixes)
	if host == "" {
		return nil
	}
	addresses := r.neighborResolver.LookupAddresses(host)
	if len(addresses) == 0 {
		return nil
	}
	return dns.FixedResponse(message.Id, question, addresses, C.DefaultDNSTTL)
}

func (r *PreferredDomainResolver) hasNeighborHost(domain string) bool {
	if r.neighborResolver == nil {
		return false
	}
	host := extractNeighborHost(domain, r.neighborSuffixes)
	if host == "" {
		return false
	}
	return len(r.neighborResolver.LookupAddresses(host)) > 0
}

func extractNeighborHost(canonical string, suffixes []string) string {
	for _, suffix := range suffixes {
		if !strings.HasSuffix(canonical, suffix) || len(canonical) <= len(suffix) {
			continue
		}
		host := canonical[:len(canonical)-len(suffix)]
		if !strings.ContainsRune(host, '.') {
			return host
		}
	}
	return ""
}
