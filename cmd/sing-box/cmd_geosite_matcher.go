package main

import (
	"regexp"
	"strings"

	"github.com/sagernet/sing-box/common/geosite"
)

type searchGeositeMatcher struct {
	domainMap   map[string]bool
	suffixList  []string
	keywordList []string
	regexList   []string
}

func newSearchGeositeMatcher(items []geosite.Item) (*searchGeositeMatcher, error) {
	options := geosite.Compile(items)
	domainMap := make(map[string]bool)
	for _, domain := range options.Domain {
		domainMap[domain] = true
	}
	rule := &searchGeositeMatcher{
		domainMap:   domainMap,
		suffixList:  options.DomainSuffix,
		keywordList: options.DomainKeyword,
		regexList:   options.DomainRegex,
	}
	return rule, nil
}

func (r *searchGeositeMatcher) Match(domain string) string {
	if r.domainMap[domain] {
		return "domain=" + domain
	}
	for _, suffix := range r.suffixList {
		if strings.HasSuffix(domain, suffix) {
			return "domain_suffix=" + suffix
		}
	}
	for _, keyword := range r.keywordList {
		if strings.Contains(domain, keyword) {
			return "domain_keyword=" + keyword
		}
	}
	for _, regexStr := range r.regexList {
		regex, err := regexp.Compile(regexStr)
		if err != nil {
			continue
		}
		if regex.MatchString(domain) {
			return "domain_regex=" + regexStr
		}
	}
	return ""
}
