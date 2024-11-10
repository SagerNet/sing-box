package rule

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
)

var _ RuleItem = (*ClashModeItem)(nil)

type ClashModeItem struct {
	clashServer adapter.ClashServer
	mode        string
}

func NewClashModeItem(clashServer adapter.ClashServer, mode string) *ClashModeItem {
	return &ClashModeItem{
		clashServer: clashServer,
		mode:        mode,
	}
}

func (r *ClashModeItem) Match(metadata *adapter.InboundContext) bool {
	if r.clashServer == nil {
		return false
	}
	return strings.EqualFold(r.clashServer.Mode(), r.mode)
}

func (r *ClashModeItem) String() string {
	return "clash_mode=" + r.mode
}
