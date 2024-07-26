package adguard

import (
	"bufio"
	"io"
	"net/netip"
	"os"
	"strconv"
	"strings"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
)

type agdguardRuleLine struct {
	ruleLine    string
	isRawDomain bool
	isExclude   bool
	isSuffix    bool
	hasStart    bool
	hasEnd      bool
	isRegexp    bool
	isImportant bool
}

func Convert(reader io.Reader) ([]option.HeadlessRule, error) {
	scanner := bufio.NewScanner(reader)
	var (
		ruleLines    []agdguardRuleLine
		ignoredLines int
	)
parseLine:
	for scanner.Scan() {
		ruleLine := scanner.Text()
		if ruleLine == "" || ruleLine[0] == '!' || ruleLine[0] == '#' {
			continue
		}
		originRuleLine := ruleLine
		if M.IsDomainName(ruleLine) {
			ruleLines = append(ruleLines, agdguardRuleLine{
				ruleLine:    ruleLine,
				isRawDomain: true,
			})
			continue
		}
		hostLine, err := parseAdGuardHostLine(ruleLine)
		if err == nil {
			if hostLine != "" {
				ruleLines = append(ruleLines, agdguardRuleLine{
					ruleLine:    hostLine,
					isRawDomain: true,
					hasStart:    true,
					hasEnd:      true,
				})
			}
			continue
		}
		if strings.HasSuffix(ruleLine, "|") {
			ruleLine = ruleLine[:len(ruleLine)-1]
		}
		var (
			isExclude   bool
			isSuffix    bool
			hasStart    bool
			hasEnd      bool
			isRegexp    bool
			isImportant bool
		)
		if !strings.HasPrefix(ruleLine, "/") && strings.Contains(ruleLine, "$") {
			params := common.SubstringAfter(ruleLine, "$")
			for _, param := range strings.Split(params, ",") {
				paramParts := strings.Split(param, "=")
				var ignored bool
				if len(paramParts) > 0 && len(paramParts) <= 2 {
					switch paramParts[0] {
					case "app", "network":
						// maybe support by package_name/process_name
					case "dnstype":
						// maybe support by query_type
					case "important":
						ignored = true
						isImportant = true
					case "dnsrewrite":
						if len(paramParts) == 2 && M.ParseAddr(paramParts[1]).IsUnspecified() {
							ignored = true
						}
					}
				}
				if !ignored {
					ignoredLines++
					log.Debug("ignored unsupported rule with modifier: ", paramParts[0], ": ", ruleLine)
					continue parseLine
				}
			}
			ruleLine = common.SubstringBefore(ruleLine, "$")
		}
		if strings.HasPrefix(ruleLine, "@@") {
			ruleLine = ruleLine[2:]
			isExclude = true
		}
		if strings.HasSuffix(ruleLine, "|") {
			ruleLine = ruleLine[:len(ruleLine)-1]
		}
		if strings.HasPrefix(ruleLine, "||") {
			ruleLine = ruleLine[2:]
			isSuffix = true
		} else if strings.HasPrefix(ruleLine, "|") {
			ruleLine = ruleLine[1:]
			hasStart = true
		}
		if strings.HasSuffix(ruleLine, "^") {
			ruleLine = ruleLine[:len(ruleLine)-1]
			hasEnd = true
		}
		if strings.HasPrefix(ruleLine, "/") && strings.HasSuffix(ruleLine, "/") {
			ruleLine = ruleLine[1 : len(ruleLine)-1]
			if ignoreIPCIDRRegexp(ruleLine) {
				ignoredLines++
				log.Debug("ignored unsupported rule with IPCIDR regexp: ", ruleLine)
				continue
			}
			isRegexp = true
		} else {
			if strings.Contains(ruleLine, "://") {
				ruleLine = common.SubstringAfter(ruleLine, "://")
			}
			if strings.Contains(ruleLine, "/") {
				ignoredLines++
				log.Debug("ignored unsupported rule with path: ", ruleLine)
				continue
			}
			if strings.Contains(ruleLine, "##") {
				ignoredLines++
				log.Debug("ignored unsupported rule with element hiding: ", ruleLine)
				continue
			}
			if strings.Contains(ruleLine, "#$#") {
				ignoredLines++
				log.Debug("ignored unsupported rule with element hiding: ", ruleLine)
				continue
			}
			var domainCheck string
			if strings.HasPrefix(ruleLine, ".") || strings.HasPrefix(ruleLine, "-") {
				domainCheck = "r" + ruleLine
			} else {
				domainCheck = ruleLine
			}
			if ruleLine == "" {
				ignoredLines++
				log.Debug("ignored unsupported rule with empty domain", originRuleLine)
				continue
			} else {
				domainCheck = strings.ReplaceAll(domainCheck, "*", "x")
				if !M.IsDomainName(domainCheck) {
					_, ipErr := parseADGuardIPCIDRLine(ruleLine)
					if ipErr == nil {
						ignoredLines++
						log.Debug("ignored unsupported rule with IPCIDR: ", ruleLine)
						continue
					}
					if M.ParseSocksaddr(domainCheck).Port != 0 {
						log.Debug("ignored unsupported rule with port: ", ruleLine)
					} else {
						log.Debug("ignored unsupported rule with invalid domain: ", ruleLine)
					}
					ignoredLines++
					continue
				}
			}
		}
		ruleLines = append(ruleLines, agdguardRuleLine{
			ruleLine:    ruleLine,
			isExclude:   isExclude,
			isSuffix:    isSuffix,
			hasStart:    hasStart,
			hasEnd:      hasEnd,
			isRegexp:    isRegexp,
			isImportant: isImportant,
		})
	}
	if len(ruleLines) == 0 {
		return nil, E.New("AdGuard rule-set is empty or all rules are unsupported")
	}
	if common.All(ruleLines, func(it agdguardRuleLine) bool {
		return it.isRawDomain
	}) {
		return []option.HeadlessRule{
			{
				Type: C.RuleTypeDefault,
				DefaultOptions: option.DefaultHeadlessRule{
					Domain: common.Map(ruleLines, func(it agdguardRuleLine) string {
						return it.ruleLine
					}),
				},
			},
		}, nil
	}
	mapDomain := func(it agdguardRuleLine) string {
		ruleLine := it.ruleLine
		if it.isSuffix {
			ruleLine = "||" + ruleLine
		} else if it.hasStart {
			ruleLine = "|" + ruleLine
		}
		if it.hasEnd {
			ruleLine += "^"
		}
		return ruleLine
	}

	importantDomain := common.Map(common.Filter(ruleLines, func(it agdguardRuleLine) bool { return it.isImportant && !it.isRegexp && !it.isExclude }), mapDomain)
	importantDomainRegex := common.Map(common.Filter(ruleLines, func(it agdguardRuleLine) bool { return it.isImportant && it.isRegexp && !it.isExclude }), mapDomain)
	importantExcludeDomain := common.Map(common.Filter(ruleLines, func(it agdguardRuleLine) bool { return it.isImportant && !it.isRegexp && it.isExclude }), mapDomain)
	importantExcludeDomainRegex := common.Map(common.Filter(ruleLines, func(it agdguardRuleLine) bool { return it.isImportant && it.isRegexp && it.isExclude }), mapDomain)
	domain := common.Map(common.Filter(ruleLines, func(it agdguardRuleLine) bool { return !it.isImportant && !it.isRegexp && !it.isExclude }), mapDomain)
	domainRegex := common.Map(common.Filter(ruleLines, func(it agdguardRuleLine) bool { return !it.isImportant && it.isRegexp && !it.isExclude }), mapDomain)
	excludeDomain := common.Map(common.Filter(ruleLines, func(it agdguardRuleLine) bool { return !it.isImportant && !it.isRegexp && it.isExclude }), mapDomain)
	excludeDomainRegex := common.Map(common.Filter(ruleLines, func(it agdguardRuleLine) bool { return !it.isImportant && it.isRegexp && it.isExclude }), mapDomain)
	currentRule := option.HeadlessRule{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultHeadlessRule{
			AdGuardDomain: domain,
			DomainRegex:   domainRegex,
		},
	}
	if len(excludeDomain) > 0 || len(excludeDomainRegex) > 0 {
		currentRule = option.HeadlessRule{
			Type: C.RuleTypeLogical,
			LogicalOptions: option.LogicalHeadlessRule{
				Mode: C.LogicalTypeAnd,
				Rules: []option.HeadlessRule{
					{
						Type: C.RuleTypeDefault,
						DefaultOptions: option.DefaultHeadlessRule{
							AdGuardDomain: excludeDomain,
							DomainRegex:   excludeDomainRegex,
							Invert:        true,
						},
					},
					currentRule,
				},
			},
		}
	}
	if len(importantDomain) > 0 || len(importantDomainRegex) > 0 {
		currentRule = option.HeadlessRule{
			Type: C.RuleTypeLogical,
			LogicalOptions: option.LogicalHeadlessRule{
				Mode: C.LogicalTypeOr,
				Rules: []option.HeadlessRule{
					{
						Type: C.RuleTypeDefault,
						DefaultOptions: option.DefaultHeadlessRule{
							AdGuardDomain: importantDomain,
							DomainRegex:   importantDomainRegex,
						},
					},
					currentRule,
				},
			},
		}
	}
	if len(importantExcludeDomain) > 0 || len(importantExcludeDomainRegex) > 0 {
		currentRule = option.HeadlessRule{
			Type: C.RuleTypeLogical,
			LogicalOptions: option.LogicalHeadlessRule{
				Mode: C.LogicalTypeAnd,
				Rules: []option.HeadlessRule{
					{
						Type: C.RuleTypeDefault,
						DefaultOptions: option.DefaultHeadlessRule{
							AdGuardDomain: importantExcludeDomain,
							DomainRegex:   importantExcludeDomainRegex,
							Invert:        true,
						},
					},
					currentRule,
				},
			},
		}
	}
	log.Info("parsed rules: ", len(ruleLines), "/", len(ruleLines)+ignoredLines)
	return []option.HeadlessRule{currentRule}, nil
}

func ignoreIPCIDRRegexp(ruleLine string) bool {
	if strings.HasPrefix(ruleLine, "(http?:\\/\\/)") {
		ruleLine = ruleLine[12:]
	} else if strings.HasPrefix(ruleLine, "(https?:\\/\\/)") {
		ruleLine = ruleLine[13:]
	} else if strings.HasPrefix(ruleLine, "^") {
		ruleLine = ruleLine[1:]
	} else {
		return false
	}
	_, parseErr := strconv.ParseUint(common.SubstringBefore(ruleLine, "\\."), 10, 8)
	return parseErr == nil
}

func parseAdGuardHostLine(ruleLine string) (string, error) {
	idx := strings.Index(ruleLine, " ")
	if idx == -1 {
		return "", os.ErrInvalid
	}
	address, err := netip.ParseAddr(ruleLine[:idx])
	if err != nil {
		return "", err
	}
	if !address.IsUnspecified() {
		return "", nil
	}
	domain := ruleLine[idx+1:]
	if !M.IsDomainName(domain) {
		return "", E.New("invalid domain name: ", domain)
	}
	return domain, nil
}

func parseADGuardIPCIDRLine(ruleLine string) (netip.Prefix, error) {
	var isPrefix bool
	if strings.HasSuffix(ruleLine, ".") {
		isPrefix = true
		ruleLine = ruleLine[:len(ruleLine)-1]
	}
	ruleStringParts := strings.Split(ruleLine, ".")
	if len(ruleStringParts) > 4 || len(ruleStringParts) < 4 && !isPrefix {
		return netip.Prefix{}, os.ErrInvalid
	}
	ruleParts := make([]uint8, 0, len(ruleStringParts))
	for _, part := range ruleStringParts {
		rulePart, err := strconv.ParseUint(part, 10, 8)
		if err != nil {
			return netip.Prefix{}, err
		}
		ruleParts = append(ruleParts, uint8(rulePart))
	}
	bitLen := len(ruleParts) * 8
	for len(ruleParts) < 4 {
		ruleParts = append(ruleParts, 0)
	}
	return netip.PrefixFrom(netip.AddrFrom4(*(*[4]byte)(ruleParts)), bitLen), nil
}
