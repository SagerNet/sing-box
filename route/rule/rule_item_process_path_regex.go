package rule

import (
	"regexp"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
)

var _ RuleItem = (*ProcessPathRegexItem)(nil)

type ProcessPathRegexItem struct {
	matchers    []*regexp.Regexp
	description string
}

func NewProcessPathRegexItem(expressions []string) (*ProcessPathRegexItem, error) {
	matchers := make([]*regexp.Regexp, 0, len(expressions))
	for i, regex := range expressions {
		matcher, err := regexp.Compile(regex)
		if err != nil {
			return nil, E.Cause(err, "parse expression ", i)
		}
		matchers = append(matchers, matcher)
	}
	description := "process_path_regex="
	eLen := len(expressions)
	if eLen == 1 {
		description += expressions[0]
	} else if eLen > 3 {
		description += F.ToString("[", strings.Join(expressions[:3], " "), "]")
	} else {
		description += F.ToString("[", strings.Join(expressions, " "), "]")
	}
	return &ProcessPathRegexItem{matchers, description}, nil
}

func (r *ProcessPathRegexItem) Match(metadata *adapter.InboundContext) bool {
	if metadata.ProcessInfo == nil || metadata.ProcessInfo.ProcessPath == "" {
		return false
	}
	for _, matcher := range r.matchers {
		if matcher.MatchString(metadata.ProcessInfo.ProcessPath) {
			return true
		}
	}
	return false
}

func (r *ProcessPathRegexItem) String() string {
	return r.description
}
