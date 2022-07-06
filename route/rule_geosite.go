package route

import (
	"strings"

	E "github.com/sagernet/sing/common/exceptions"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/geosite"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

var _ RuleItem = (*GeositeItem)(nil)

type GeositeItem struct {
	router  adapter.Router
	logger  log.Logger
	codes   []string
	matcher *DefaultRule
}

func NewGeositeItem(router adapter.Router, logger log.Logger, codes []string) *GeositeItem {
	return &GeositeItem{
		router: router,
		logger: logger,
		codes:  codes,
	}
}

func (r *GeositeItem) Update() error {
	geositeReader := r.router.GeositeReader()
	if geositeReader == nil {
		return E.New("geosite reader is not initialized")
	}
	var subRules []option.DefaultRule
	for _, code := range r.codes {
		items, err := geositeReader.Read(code)
		if err != nil {
			return E.Cause(err, "read geosite")
		}
		subRules = append(subRules, geosite.Compile(items))
	}
	matcherRule := geosite.Merge(subRules)
	matcher, err := NewDefaultRule(r.router, r.logger, matcherRule)
	if err != nil {
		return E.Cause(err, "compile geosite")
	}
	r.matcher = matcher
	return nil
}

func (r *GeositeItem) Match(metadata *adapter.InboundContext) bool {
	if r.matcher == nil {
		return false
	}
	return r.matcher.Match(metadata)
}

func (r *GeositeItem) String() string {
	description := "geosite="
	cLen := len(r.codes)
	if cLen == 1 {
		description += r.codes[0]
	} else if cLen > 3 {
		description += "[" + strings.Join(r.codes[:3], " ") + "...]"
	} else {
		description += "[" + strings.Join(r.codes, " ") + "]"
	}
	return description
}
