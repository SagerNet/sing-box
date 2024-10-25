package route

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"

	"go4.org/netipx"
)

func NewRuleSet(ctx context.Context, router adapter.Router, logger logger.ContextLogger, options option.RuleSet) (adapter.RuleSet, error) {
	switch options.Type {
	case C.RuleSetTypeInline, C.RuleSetTypeLocal, "":
		return NewLocalRuleSet(ctx, router, logger, options)
	case C.RuleSetTypeRemote:
		return NewRemoteRuleSet(ctx, router, logger, options), nil
	default:
		return nil, E.New("unknown rule-set type: ", options.Type)
	}
}

func extractIPSetFromRule(rawRule adapter.HeadlessRule) []*netipx.IPSet {
	switch rule := rawRule.(type) {
	case *DefaultHeadlessRule:
		return common.FlatMap(rule.destinationIPCIDRItems, func(rawItem RuleItem) []*netipx.IPSet {
			switch item := rawItem.(type) {
			case *IPCIDRItem:
				return []*netipx.IPSet{item.ipSet}
			default:
				return nil
			}
		})
	case *LogicalHeadlessRule:
		return common.FlatMap(rule.rules, extractIPSetFromRule)
	default:
		panic("unexpected rule type")
	}
}
