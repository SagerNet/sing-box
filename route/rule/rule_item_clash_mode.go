package rule

import (
	"context"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/experimental/clashmode"
	"github.com/sagernet/sing/service"
)

var _ RuleItem = (*ClashModeItem)(nil)

type ClashModeItem struct {
	ctx       context.Context
	clashMode *clashmode.Manager
	mode      string
}

func NewClashModeItem(ctx context.Context, mode string) *ClashModeItem {
	return &ClashModeItem{
		ctx:  ctx,
		mode: mode,
	}
}

func (r *ClashModeItem) Start() error {
	r.clashMode = service.PtrFromContext[clashmode.Manager](r.ctx)
	return nil
}

func (r *ClashModeItem) Match(metadata *adapter.InboundContext) bool {
	if r.clashMode == nil {
		return false
	}
	return strings.EqualFold(r.clashMode.Mode(), r.mode)
}

func (r *ClashModeItem) String() string {
	return "clash_mode=" + r.mode
}
