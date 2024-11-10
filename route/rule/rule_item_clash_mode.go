package rule

import (
	"context"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/service"
)

var _ RuleItem = (*ClashModeItem)(nil)

type ClashModeItem struct {
	ctx         context.Context
	clashServer adapter.ClashServer
	mode        string
}

func NewClashModeItem(ctx context.Context, mode string) *ClashModeItem {
	return &ClashModeItem{
		ctx:  ctx,
		mode: mode,
	}
}

func (r *ClashModeItem) Start() error {
	r.clashServer = service.FromContext[adapter.ClashServer](r.ctx)
	return nil
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
