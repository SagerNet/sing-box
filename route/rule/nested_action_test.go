package rule

import (
	"context"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json"

	"github.com/stretchr/testify/require"
)

func TestNewRulePreservesImplicitTopLevelDefaultAction(t *testing.T) {
	t.Parallel()

	var options option.Rule
	err := json.UnmarshalContext(context.Background(), []byte(`{
		"domain": "example.com"
	}`), &options)
	require.NoError(t, err)

	rule, err := NewRule(context.Background(), log.NewNOPFactory().NewLogger("router"), options, false)
	require.NoError(t, err)
	require.NotNil(t, rule.Action())
	require.Equal(t, C.RuleActionTypeRoute, rule.Action().Type())
}

func TestNewRuleAllowsNestedRuleWithoutAction(t *testing.T) {
	t.Parallel()

	var options option.Rule
	err := json.UnmarshalContext(context.Background(), []byte(`{
		"type": "logical",
		"mode": "and",
		"rules": [
			{"domain": "example.com"}
		]
	}`), &options)
	require.NoError(t, err)

	rule, err := NewRule(context.Background(), log.NewNOPFactory().NewLogger("router"), options, false)
	require.NoError(t, err)
	require.NotNil(t, rule.Action())
	require.Equal(t, C.RuleActionTypeRoute, rule.Action().Type())
}

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
	require.ErrorContains(t, err, routeRuleActionNestedUnsupportedMessage)
}

func TestNewDNSRulePreservesImplicitTopLevelDefaultAction(t *testing.T) {
	t.Parallel()

	var options option.DNSRule
	err := json.UnmarshalContext(context.Background(), []byte(`{
		"domain": "example.com"
	}`), &options)
	require.NoError(t, err)

	rule, err := NewDNSRule(context.Background(), log.NewNOPFactory().NewLogger("dns"), options, false, false)
	require.NoError(t, err)
	require.NotNil(t, rule.Action())
	require.Equal(t, C.RuleActionTypeRoute, rule.Action().Type())
}

func TestNewDNSRuleAllowsNestedRuleWithoutAction(t *testing.T) {
	t.Parallel()

	var options option.DNSRule
	err := json.UnmarshalContext(context.Background(), []byte(`{
		"type": "logical",
		"mode": "and",
		"rules": [
			{"domain": "example.com"}
		]
	}`), &options)
	require.NoError(t, err)

	rule, err := NewDNSRule(context.Background(), log.NewNOPFactory().NewLogger("dns"), options, false, false)
	require.NoError(t, err)
	require.NotNil(t, rule.Action())
	require.Equal(t, C.RuleActionTypeRoute, rule.Action().Type())
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
	require.ErrorContains(t, err, dnsRuleActionNestedUnsupportedMessage)
}
