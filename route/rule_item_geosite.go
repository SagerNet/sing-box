package route

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
)

var _ RuleItem = (*GeositeItem)(nil)

type GeositeItem struct {
	router   adapter.Router
	logger   log.ContextLogger
	codes    []string
	matchers []adapter.Rule
}

func NewGeositeItem(router adapter.Router, logger log.ContextLogger, codes []string) *GeositeItem {
	return &GeositeItem{
		router: router,
		logger: logger,
		codes:  codes,
	}
}

func (r *GeositeItem) Update() error {
	matchers := make([]adapter.Rule, 0, len(r.codes))
	for _, code := range r.codes {
		matcher, err := r.router.LoadGeosite(code)
		if err != nil {
			return E.Cause(err, "read geosite")
		}
		matchers = append(matchers, matcher)
	}
	r.matchers = matchers
	return nil
}

func (r *GeositeItem) Match(metadata *adapter.InboundContext) bool {
	for _, matcher := range r.matchers {
		if matcher.Match(metadata) {
			return true
		}
	}
	return false
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
