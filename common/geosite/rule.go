package geosite

import "github.com/sagernet/sing-box/option"

type ItemType = uint8

const (
	RuleTypeDomain ItemType = iota
	RuleTypeDomainSuffix
	RuleTypeDomainKeyword
	RuleTypeDomainRegex
)

type Item struct {
	Type  ItemType
	Value string
}

func Compile(code []Item) option.DefaultRule {
	var domainLength int
	var domainSuffixLength int
	var domainKeywordLength int
	var domainRegexLength int
	for _, item := range code {
		switch item.Type {
		case RuleTypeDomain:
			domainLength++
		case RuleTypeDomainSuffix:
			domainSuffixLength++
		case RuleTypeDomainKeyword:
			domainKeywordLength++
		case RuleTypeDomainRegex:
			domainRegexLength++
		}
	}
	var codeRule option.DefaultRule
	if domainLength > 0 {
		codeRule.Domain = make([]string, 0, domainLength)
	}
	if domainSuffixLength > 0 {
		codeRule.DomainSuffix = make([]string, 0, domainSuffixLength)
	}
	if domainKeywordLength > 0 {
		codeRule.DomainKeyword = make([]string, 0, domainKeywordLength)
	}
	if domainRegexLength > 0 {
		codeRule.DomainRegex = make([]string, 0, domainRegexLength)
	}
	for _, item := range code {
		switch item.Type {
		case RuleTypeDomain:
			codeRule.Domain = append(codeRule.Domain, item.Value)
		case RuleTypeDomainSuffix:
			codeRule.DomainSuffix = append(codeRule.DomainSuffix, item.Value)
		case RuleTypeDomainKeyword:
			codeRule.DomainKeyword = append(codeRule.DomainKeyword, item.Value)
		case RuleTypeDomainRegex:
			codeRule.DomainRegex = append(codeRule.DomainRegex, item.Value)
		}
	}
	return codeRule
}
