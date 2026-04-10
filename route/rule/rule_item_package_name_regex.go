package rule

import (
	"regexp"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
)

var _ RuleItem = (*PackageNameRegexItem)(nil)

type PackageNameRegexItem struct {
	matchers    []*regexp.Regexp
	description string
}

func NewPackageNameRegexItem(expressions []string) (*PackageNameRegexItem, error) {
	matchers := make([]*regexp.Regexp, 0, len(expressions))
	for i, regex := range expressions {
		matcher, err := regexp.Compile(regex)
		if err != nil {
			return nil, E.Cause(err, "parse expression ", i)
		}
		matchers = append(matchers, matcher)
	}
	description := "package_name_regex="
	eLen := len(expressions)
	if eLen == 1 {
		description += expressions[0]
	} else if eLen > 3 {
		description += F.ToString("[", strings.Join(expressions[:3], " "), "]")
	} else {
		description += F.ToString("[", strings.Join(expressions, " "), "]")
	}
	return &PackageNameRegexItem{matchers, description}, nil
}

func (r *PackageNameRegexItem) Match(metadata *adapter.InboundContext) bool {
	if metadata.ProcessInfo == nil || len(metadata.ProcessInfo.AndroidPackageNames) == 0 {
		return false
	}
	for _, matcher := range r.matchers {
		for _, packageName := range metadata.ProcessInfo.AndroidPackageNames {
			if matcher.MatchString(packageName) {
				return true
			}
		}
	}
	return false
}

func (r *PackageNameRegexItem) String() string {
	return r.description
}
