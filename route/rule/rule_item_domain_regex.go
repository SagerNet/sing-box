package rule

import (
	"regexp"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
)

var _ RuleItem = (*DomainRegexItem)(nil)

type DomainRegexItem struct {
	matchers    []*regexp.Regexp
	description string
}

func NewDomainRegexItem(expressions []string) (*DomainRegexItem, error) {
	matchers := make([]*regexp.Regexp, 0, len(expressions))
	for i, regex := range expressions {
		matcher, err := regexp.Compile(regex)
		if err != nil {
			return nil, E.Cause(err, "parse expression ", i)
		}
		matchers = append(matchers, matcher)
	}
	description := "domain_regex="
	eLen := len(expressions)
	if eLen == 1 {
		description += expressions[0]
	} else if eLen > 3 {
		description += F.ToString("[", strings.Join(expressions[:3], " "), "]")
	} else {
		description += F.ToString("[", strings.Join(expressions, " "), "]")
	}
	return &DomainRegexItem{matchers, description}, nil
}

func (r *DomainRegexItem) Match(metadata *adapter.InboundContext) bool {
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
	for _, matcher := range r.matchers {
		if matcher.MatchString(domainHost) {
			return true
		}
	}
	return false
}

func (r *DomainRegexItem) String() string {
	return r.description
}
