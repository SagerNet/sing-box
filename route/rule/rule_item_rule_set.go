package rule

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
)

var _ RuleItem = (*RuleSetItem)(nil)

type RuleSetItem struct {
	router            adapter.Router
	tagList           []string
	setList           []adapter.RuleSet
	ipCidrMatchSource bool
	ipCidrAcceptEmpty bool
}

func NewRuleSetItem(router adapter.Router, tagList []string, ipCIDRMatchSource bool, ipCidrAcceptEmpty bool) *RuleSetItem {
	return &RuleSetItem{
		router:            router,
		tagList:           tagList,
		ipCidrMatchSource: ipCIDRMatchSource,
		ipCidrAcceptEmpty: ipCidrAcceptEmpty,
	}
}

func (r *RuleSetItem) Start() error {
	for _, tag := range r.tagList {
		ruleSet, loaded := r.router.RuleSet(tag)
		if !loaded {
			return E.New("rule-set not found: ", tag)
		}
		ruleSet.IncRef()
		r.setList = append(r.setList, ruleSet)
	}
	return nil
}

func (r *RuleSetItem) Match(metadata *adapter.InboundContext) bool {
	metadata.IPCIDRMatchSource = r.ipCidrMatchSource
	metadata.IPCIDRAcceptEmpty = r.ipCidrAcceptEmpty
	for _, ruleSet := range r.setList {
		if ruleSet.Match(metadata) {
			return true
		}
	}
	return false
}

func (r *RuleSetItem) ContainsDestinationIPCIDRRule() bool {
	if r.ipCidrMatchSource {
		return false
	}
	return common.Any(r.setList, func(ruleSet adapter.RuleSet) bool {
		return ruleSet.Metadata().ContainsIPCIDRRule
	})
}

func (r *RuleSetItem) String() string {
	if len(r.tagList) == 1 {
		return F.ToString("rule_set=", r.tagList[0])
	} else {
		return F.ToString("rule_set=[", strings.Join(r.tagList, " "), "]")
	}
}
