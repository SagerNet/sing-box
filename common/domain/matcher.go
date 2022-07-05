package domain

import (
	"sort"
	"unicode/utf8"
)

type Matcher struct {
	set *succinctSet
}

func NewMatcher(domains []string, domainSuffix []string) *Matcher {
	domainList := make([]string, 0, len(domains)+len(domainSuffix))
	seen := make(map[string]bool, len(domainList))
	for _, domain := range domainSuffix {
		if seen[domain] {
			continue
		}
		seen[domain] = true
		domainList = append(domainList, reverseDomainSuffix(domain))
	}
	for _, domain := range domains {
		if seen[domain] {
			continue
		}
		seen[domain] = true
		domainList = append(domainList, reverseDomain(domain))
	}
	sort.Strings(domainList)
	return &Matcher{
		newSuccinctSet(domainList),
	}
}

func (m *Matcher) Match(domain string) bool {
	return m.set.Has(reverseDomain(domain))
}

func reverseDomain(domain string) string {
	l := len(domain)
	b := make([]byte, l)
	for i := 0; i < l; {
		r, n := utf8.DecodeRuneInString(domain[i:])
		i += n
		utf8.EncodeRune(b[l-i:], r)
	}
	return string(b)
}

func reverseDomainSuffix(domain string) string {
	l := len(domain)
	b := make([]byte, l+1)
	for i := 0; i < l; {
		r, n := utf8.DecodeRuneInString(domain[i:])
		i += n
		utf8.EncodeRune(b[l-i:], r)
	}
	b[l] = prefixLabel
	return string(b)
}
