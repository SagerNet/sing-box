package rule

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
)

var _ RuleItem = (*SourceHostnameItem)(nil)

type SourceHostnameItem struct {
	hostnames   []string
	hostnameMap map[string]bool
}

func NewSourceHostnameItem(hostnameList []string) *SourceHostnameItem {
	rule := &SourceHostnameItem{
		hostnames:   hostnameList,
		hostnameMap: make(map[string]bool),
	}
	for _, hostname := range hostnameList {
		rule.hostnameMap[hostname] = true
	}
	return rule
}

func (r *SourceHostnameItem) Match(metadata *adapter.InboundContext) bool {
	if metadata.SourceHostname == "" {
		return false
	}
	return r.hostnameMap[metadata.SourceHostname]
}

func (r *SourceHostnameItem) String() string {
	var description string
	if len(r.hostnames) == 1 {
		description = "source_hostname=" + r.hostnames[0]
	} else {
		description = "source_hostname=[" + strings.Join(r.hostnames, " ") + "]"
	}
	return description
}
