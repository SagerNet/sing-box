package route

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common"
)

var _ RuleItem = (*ClashModeItem)(nil)

type ClashModeItem struct {
	router adapter.Router
	modes  []string
}

func NewClashModeItem(router adapter.Router, modes []string) *ClashModeItem {
	return &ClashModeItem{
		router: router,
		modes:  modes,
	}
}

func (r *ClashModeItem) Match(metadata *adapter.InboundContext) bool {
	clashServer := r.router.ClashServer()
	if clashServer == nil {
		return false
	}
	return common.Any(r.modes, func(mode string) bool {
		return strings.EqualFold(clashServer.Mode(), mode)
	})
}

func (r *ClashModeItem) String() string {
	modeStr := r.modes[0]
	if len(r.modes) > 1 {
		modeStr = "[" + strings.Join(r.modes, ", ") + "]"
	}
	return "clash_mode=" + modeStr
}
