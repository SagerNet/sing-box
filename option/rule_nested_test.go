package option

import (
	"context"
	"testing"

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
