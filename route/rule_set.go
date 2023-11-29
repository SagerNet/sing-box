package route

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
)

func NewRuleSet(ctx context.Context, router adapter.Router, logger logger.ContextLogger, options option.RuleSet) (adapter.RuleSet, error) {
	switch options.Type {
	case C.RuleSetTypeLocal:
		return NewLocalRuleSet(router, options)
	case C.RuleSetTypeRemote:
		return NewRemoteRuleSet(ctx, router, logger, options), nil
	default:
		return nil, E.New("unknown rule set type: ", options.Type)
	}
}
