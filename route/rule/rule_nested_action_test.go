package rule

import (
	"context"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"

	"github.com/stretchr/testify/require"
)

func TestNewRuleRejectsNestedRuleAction(t *testing.T) {
	t.Parallel()

	_, err := NewRule(context.Background(), log.NewNOPFactory().NewLogger("router"), option.Rule{
		Type: C.RuleTypeLogical,
		LogicalOptions: option.LogicalRule{
			RawLogicalRule: option.RawLogicalRule{
				Mode: C.LogicalTypeAnd,
				Rules: []option.Rule{{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RuleAction: option.RuleAction{
							Action: C.RuleActionTypeRoute,
							RouteOptions: option.RouteActionOptions{
								Outbound: "direct",
							},
						},
					},
				}},
			},
		},
	}, false)
	require.ErrorContains(t, err, option.RouteRuleActionNestedUnsupportedMessage)
}

func TestNewDNSRuleRejectsNestedRuleAction(t *testing.T) {
	t.Parallel()

	_, err := NewDNSRule(context.Background(), log.NewNOPFactory().NewLogger("dns"), option.DNSRule{
		Type: C.RuleTypeLogical,
		LogicalOptions: option.LogicalDNSRule{
			RawLogicalDNSRule: option.RawLogicalDNSRule{
				Mode: C.LogicalTypeAnd,
				Rules: []option.DNSRule{{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultDNSRule{
						DNSRuleAction: option.DNSRuleAction{
							Action: C.RuleActionTypeRoute,
							RouteOptions: option.DNSRouteActionOptions{
								Server: "default",
							},
						},
					},
				}},
			},
			DNSRuleAction: option.DNSRuleAction{
				Action: C.RuleActionTypeRoute,
				RouteOptions: option.DNSRouteActionOptions{
					Server: "default",
				},
			},
		},
	}, true, false)
	require.ErrorContains(t, err, option.DNSRuleActionNestedUnsupportedMessage)
}

func TestNewDNSRuleRejectsReplyRejectMethod(t *testing.T) {
	t.Parallel()

	_, err := NewDNSRule(context.Background(), log.NewNOPFactory().NewLogger("dns"), option.DNSRule{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				Domain: []string{"example.com"},
			},
			DNSRuleAction: option.DNSRuleAction{
				Action: C.RuleActionTypeReject,
				RejectOptions: option.RejectActionOptions{
					Method: C.RuleActionRejectMethodReply,
				},
			},
		},
	}, false, false)
	require.ErrorContains(t, err, "reject method `reply` is not supported for DNS rules")
}
