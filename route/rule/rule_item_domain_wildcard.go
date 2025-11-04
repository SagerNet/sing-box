package rule

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
)

var _ RuleItem = (*DomainWildcardItem)(nil)

type DomainWildcardItem struct {
	wildcards []string
}

func NewDomainWildcardItem(wildcards []string) *DomainWildcardItem {
	// lowercase all wildcards for case-insensitive matching
	for i, w := range wildcards {
		wildcards[i] = strings.ToLower(w)
	}
	return &DomainWildcardItem{wildcards}
}

func (r *DomainWildcardItem) Match(metadata *adapter.InboundContext) bool {
	var domainHost string
	if metadata.Domain != "" {
		domainHost = metadata.Domain
	} else {
		domainHost = metadata.Destination.Fqdn
	}
	if domainHost == "" {
		return false
	}
	domainHost = strings.ToLower(domainHost)
	for _, wildcard := range r.wildcards {
		if matchTwoPointers(domainHost, wildcard) {
			return true
		}
	}
	return false
}

func (r *DomainWildcardItem) String() string {
	kLen := len(r.wildcards)
	if kLen == 1 {
		return "domain_wildcard=" + r.wildcards[0]
	} else if kLen > 3 {
		return "domain_wildcard=[" + strings.Join(r.wildcards[:3], " ") + "...]"
	} else {
		return "domain_wildcard=[" + strings.Join(r.wildcards, " ") + "]"
	}
}

// matchTwoPointers checks if the domain matches the pattern with wildcards.
// The pattern can contain '?' which matches any single character and '*' which matches any sequence of characters (including the empty sequence).
func matchTwoPointers(domain, pattern string) bool {
	si, pi := 0, 0
	star, match := -1, 0

	for si < len(domain) {
		if pi < len(pattern) && (pattern[pi] == domain[si] || pattern[pi] == '?') {
			si++
			pi++
		} else if pi < len(pattern) && pattern[pi] == '*' {
			star = pi
			match = si
			pi++
		} else if star != -1 {
			pi = star + 1
			match++
			si = match
		} else {
			return false
		}
	}
	for pi < len(pattern) && pattern[pi] == '*' {
		pi++
	}
	return pi == len(pattern)
}
