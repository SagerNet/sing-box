package rule

import (
	"reflect"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func ValidateNoNestedRuleActions(rule option.Rule) error {
	return validateNoNestedRuleActions(rule, false)
}

func ValidateNoNestedDNSRuleActions(rule option.DNSRule) error {
	return validateNoNestedDNSRuleActions(rule, false)
}

func validateNoNestedRuleActions(rule option.Rule, nested bool) error {
	if nested && ruleHasConfiguredAction(rule) {
		return E.New(option.RouteRuleActionNestedUnsupportedMessage)
	}
	if rule.Type != C.RuleTypeLogical {
		return nil
	}
	for i, subRule := range rule.LogicalOptions.Rules {
		err := validateNoNestedRuleActions(subRule, true)
		if err != nil {
			return E.Cause(err, "sub rule[", i, "]")
		}
	}
	return nil
}

func validateNoNestedDNSRuleActions(rule option.DNSRule, nested bool) error {
	if nested && dnsRuleHasConfiguredAction(rule) {
		return E.New(option.DNSRuleActionNestedUnsupportedMessage)
	}
	if rule.Type != C.RuleTypeLogical {
		return nil
	}
	for i, subRule := range rule.LogicalOptions.Rules {
		err := validateNoNestedDNSRuleActions(subRule, true)
		if err != nil {
			return E.Cause(err, "sub rule[", i, "]")
		}
	}
	return nil
}

func ruleHasConfiguredAction(rule option.Rule) bool {
	switch rule.Type {
	case "", C.RuleTypeDefault:
		return !reflect.DeepEqual(rule.DefaultOptions.RuleAction, option.RuleAction{})
	case C.RuleTypeLogical:
		return !reflect.DeepEqual(rule.LogicalOptions.RuleAction, option.RuleAction{})
	default:
		return false
	}
}

func dnsRuleHasConfiguredAction(rule option.DNSRule) bool {
	switch rule.Type {
	case "", C.RuleTypeDefault:
		return !reflect.DeepEqual(rule.DefaultOptions.DNSRuleAction, option.DNSRuleAction{})
	case C.RuleTypeLogical:
		return !reflect.DeepEqual(rule.LogicalOptions.DNSRuleAction, option.DNSRuleAction{})
	default:
		return false
	}
}
