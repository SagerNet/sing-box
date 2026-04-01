package option

import (
	"context"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common/json"

	"github.com/stretchr/testify/require"
)

func TestRuleRejectsNestedDefaultRuleAction(t *testing.T) {
	t.Parallel()

	var rule Rule
	err := json.UnmarshalContext(context.Background(), []byte(`{
		"type": "logical",
		"mode": "and",
		"rules": [
			{"domain": "example.com", "outbound": "direct"}
		]
	}`), &rule)
	require.ErrorContains(t, err, RouteRuleActionNestedUnsupportedMessage)
}

func TestRuleRejectsNestedLogicalRuleAction(t *testing.T) {
	t.Parallel()

	var rule Rule
	err := json.UnmarshalContext(context.Background(), []byte(`{
		"type": "logical",
		"mode": "and",
		"rules": [
			{
				"type": "logical",
				"mode": "or",
				"action": "route",
				"outbound": "direct",
				"rules": [{"domain": "example.com"}]
			}
		]
	}`), &rule)
	require.ErrorContains(t, err, RouteRuleActionNestedUnsupportedMessage)
}

func TestRuleRejectsNestedDefaultRuleZeroValueOutbound(t *testing.T) {
	t.Parallel()

	var rule Rule
	err := json.UnmarshalContext(context.Background(), []byte(`{
		"type": "logical",
		"mode": "and",
		"rules": [
			{"domain": "example.com", "outbound": ""}
		]
	}`), &rule)
	require.ErrorContains(t, err, RouteRuleActionNestedUnsupportedMessage)
}

func TestRuleRejectsNestedDefaultRuleZeroValueRouteOption(t *testing.T) {
	t.Parallel()

	var rule Rule
	err := json.UnmarshalContext(context.Background(), []byte(`{
		"type": "logical",
		"mode": "and",
		"rules": [
			{"domain": "example.com", "udp_connect": false}
		]
	}`), &rule)
	require.ErrorContains(t, err, RouteRuleActionNestedUnsupportedMessage)
}

func TestRuleRejectsNestedLogicalRuleZeroValueAction(t *testing.T) {
	t.Parallel()

	var rule Rule
	err := json.UnmarshalContext(context.Background(), []byte(`{
		"type": "logical",
		"mode": "and",
		"rules": [
			{
				"type": "logical",
				"mode": "or",
				"action": "",
				"rules": [{"domain": "example.com"}]
			}
		]
	}`), &rule)
	require.ErrorContains(t, err, RouteRuleActionNestedUnsupportedMessage)
}

func TestRuleRejectsNestedLogicalRuleZeroValueRouteOption(t *testing.T) {
	t.Parallel()

	var rule Rule
	err := json.UnmarshalContext(context.Background(), []byte(`{
		"type": "logical",
		"mode": "and",
		"rules": [
			{
				"type": "logical",
				"mode": "or",
				"override_port": 0,
				"rules": [{"domain": "example.com"}]
			}
		]
	}`), &rule)
	require.ErrorContains(t, err, RouteRuleActionNestedUnsupportedMessage)
}

func TestRuleAllowsTopLevelLogicalAction(t *testing.T) {
	t.Parallel()

	var rule Rule
	err := json.UnmarshalContext(context.Background(), []byte(`{
		"type": "logical",
		"mode": "and",
		"outbound": "direct",
		"rules": [{"domain": "example.com"}]
	}`), &rule)
	require.NoError(t, err)
	require.Equal(t, C.RuleActionTypeRoute, rule.LogicalOptions.Action)
	require.Equal(t, "direct", rule.LogicalOptions.RouteOptions.Outbound)
}

func TestRuleLeavesUnknownNestedKeysToNormalValidation(t *testing.T) {
	t.Parallel()

	var rule Rule
	err := json.UnmarshalContext(context.Background(), []byte(`{
		"type": "logical",
		"mode": "and",
		"rules": [
			{"domain": "example.com", "foo": "bar"}
		]
	}`), &rule)
	require.ErrorContains(t, err, "unknown field")
	require.NotContains(t, err.Error(), RouteRuleActionNestedUnsupportedMessage)
}

func TestDNSRuleRejectsNestedDefaultRuleAction(t *testing.T) {
	t.Parallel()

	var rule DNSRule
	err := json.UnmarshalContext(context.Background(), []byte(`{
		"type": "logical",
		"mode": "and",
		"rules": [
			{"domain": "example.com", "server": "default"}
		]
	}`), &rule)
	require.ErrorContains(t, err, DNSRuleActionNestedUnsupportedMessage)
}

func TestDNSRuleRejectsNestedLogicalRuleAction(t *testing.T) {
	t.Parallel()

	var rule DNSRule
	err := json.UnmarshalContext(context.Background(), []byte(`{
		"type": "logical",
		"mode": "and",
		"rules": [
			{
				"type": "logical",
				"mode": "or",
				"action": "route",
				"server": "default",
				"rules": [{"domain": "example.com"}]
			}
		]
	}`), &rule)
	require.ErrorContains(t, err, DNSRuleActionNestedUnsupportedMessage)
}

func TestDNSRuleRejectsNestedDefaultRuleZeroValueServer(t *testing.T) {
	t.Parallel()

	var rule DNSRule
	err := json.UnmarshalContext(context.Background(), []byte(`{
		"type": "logical",
		"mode": "and",
		"rules": [
			{"domain": "example.com", "server": ""}
		]
	}`), &rule)
	require.ErrorContains(t, err, DNSRuleActionNestedUnsupportedMessage)
}

func TestDNSRuleRejectsNestedDefaultRuleZeroValueRouteOption(t *testing.T) {
	t.Parallel()

	var rule DNSRule
	err := json.UnmarshalContext(context.Background(), []byte(`{
		"type": "logical",
		"mode": "and",
		"rules": [
			{"domain": "example.com", "disable_cache": false}
		]
	}`), &rule)
	require.ErrorContains(t, err, DNSRuleActionNestedUnsupportedMessage)
}

func TestDNSRuleRejectsNestedLogicalRuleZeroValueAction(t *testing.T) {
	t.Parallel()

	var rule DNSRule
	err := json.UnmarshalContext(context.Background(), []byte(`{
		"type": "logical",
		"mode": "and",
		"rules": [
			{
				"type": "logical",
				"mode": "or",
				"action": "",
				"rules": [{"domain": "example.com"}]
			}
		]
	}`), &rule)
	require.ErrorContains(t, err, DNSRuleActionNestedUnsupportedMessage)
}

func TestDNSRuleRejectsNestedLogicalRuleZeroValueRouteOption(t *testing.T) {
	t.Parallel()

	var rule DNSRule
	err := json.UnmarshalContext(context.Background(), []byte(`{
		"type": "logical",
		"mode": "and",
		"rules": [
			{
				"type": "logical",
				"mode": "or",
				"disable_cache": false,
				"rules": [{"domain": "example.com"}]
			}
		]
	}`), &rule)
	require.ErrorContains(t, err, DNSRuleActionNestedUnsupportedMessage)
}

func TestDNSRuleAllowsTopLevelLogicalAction(t *testing.T) {
	t.Parallel()

	var rule DNSRule
	err := json.UnmarshalContext(context.Background(), []byte(`{
		"type": "logical",
		"mode": "and",
		"server": "default",
		"rules": [{"domain": "example.com"}]
	}`), &rule)
	require.NoError(t, err)
	require.Equal(t, C.RuleActionTypeRoute, rule.LogicalOptions.Action)
	require.Equal(t, "default", rule.LogicalOptions.RouteOptions.Server)
}

func TestDNSRuleLeavesUnknownNestedKeysToNormalValidation(t *testing.T) {
	t.Parallel()

	var rule DNSRule
	err := json.UnmarshalContext(context.Background(), []byte(`{
		"type": "logical",
		"mode": "and",
		"rules": [
			{"domain": "example.com", "foo": "bar"}
		]
	}`), &rule)
	require.ErrorContains(t, err, "unknown field")
	require.NotContains(t, err.Error(), DNSRuleActionNestedUnsupportedMessage)
}
