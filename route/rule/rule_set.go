package rule

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

func NewRuleSet(ctx context.Context, logger logger.ContextLogger, options option.RuleSet) (adapter.RuleSet, error) {
	switch options.Type {
	case C.RuleSetTypeInline, C.RuleSetTypeLocal, "":
		return NewLocalRuleSet(ctx, logger, options)
	case C.RuleSetTypeRemote:
		return NewRemoteRuleSet(ctx, logger, options), nil
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

func HasHeadlessRule(rules []option.HeadlessRule, cond func(rule option.DefaultHeadlessRule) bool) bool {
	for _, rule := range rules {
		switch rule.Type {
		case C.RuleTypeDefault:
			if cond(rule.DefaultOptions) {
				return true
			}
		case C.RuleTypeLogical:
			if HasHeadlessRule(rule.LogicalOptions.Rules, cond) {
				return true
			}
		}
	}
	return false
}

func isProcessHeadlessRule(rule option.DefaultHeadlessRule) bool {
	return len(rule.ProcessName) > 0 || len(rule.ProcessPath) > 0 || len(rule.ProcessPathRegex) > 0 || len(rule.PackageName) > 0
}

func isWIFIHeadlessRule(rule option.DefaultHeadlessRule) bool {
	return len(rule.WIFISSID) > 0 || len(rule.WIFIBSSID) > 0
}

func isIPCIDRHeadlessRule(rule option.DefaultHeadlessRule) bool {
	return len(rule.IPCIDR) > 0 || rule.IPSet != nil
}
