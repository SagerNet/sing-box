package domain

import "unicode/utf8"

type Matcher struct {
	set *succinctSet
}

func NewMatcher(domains []string, domainSuffix []string) *Matcher {
	var domainList []string
	for _, domain := range domains {
		domainList = append(domainList, reverseDomain(domain))
	}
	for _, domain := range domainSuffix {
		domainList = append(domainList, reverseDomainSuffix(domain))
	}
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
