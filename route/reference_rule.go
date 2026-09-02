package route

import (
	"reflect"
	"strings"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

type staticMatch uint8

const (
	staticMatchUnknown staticMatch = iota
	staticMatchNever
	staticMatchAlways
)

func (m staticMatch) invert() staticMatch {
	switch m {
	case staticMatchNever:
		return staticMatchAlways
	case staticMatchAlways:
		return staticMatchNever
	default:
		return staticMatchUnknown
	}
}

func evaluateClashMode(clashMode string, mode string, invert bool, otherConditions bool) staticMatch {
	match := staticMatchUnknown
	if clashMode != "" && !strings.EqualFold(clashMode, mode) {
		match = staticMatchNever
	} else if !otherConditions {
		match = staticMatchAlways
	}
	if invert {
		match = match.invert()
	}
	return match
}

func evaluateLogical(matches []staticMatch, logicalMode string, invert bool) staticMatch {
	var match staticMatch
	if logicalMode == C.LogicalTypeAnd {
		match = staticMatchAlways
	} else {
		match = staticMatchNever
	}
	for _, ruleMatch := range matches {
		if logicalMode == C.LogicalTypeAnd && ruleMatch == staticMatchNever {
			match = staticMatchNever
			break
		}
		if logicalMode == C.LogicalTypeOr && ruleMatch == staticMatchAlways {
			match = staticMatchAlways
			break
		}
		if ruleMatch == staticMatchUnknown {
			match = staticMatchUnknown
		}
	}
	if invert {
		match = match.invert()
	}
	return match
}

func evaluateRule(rule option.Rule, mode string) staticMatch {
	switch rule.Type {
	case C.RuleTypeDefault:
		conditions := rule.DefaultOptions.RawDefaultRule
		conditions.ClashMode = ""
		conditions.Invert = false
		return evaluateClashMode(rule.DefaultOptions.ClashMode, mode, rule.DefaultOptions.Invert, !reflect.DeepEqual(conditions, option.RawDefaultRule{}))
	case C.RuleTypeLogical:
		matches := make([]staticMatch, 0, len(rule.LogicalOptions.Rules))
		for _, subRule := range rule.LogicalOptions.Rules {
			matches = append(matches, evaluateRule(subRule, mode))
		}
		return evaluateLogical(matches, rule.LogicalOptions.Mode, rule.LogicalOptions.Invert)
	default:
		return staticMatchUnknown
	}
}

func evaluateDNSRule(rule option.DNSRule, mode string) staticMatch {
	switch rule.Type {
	case C.RuleTypeDefault:
		conditions := rule.DefaultOptions.RawDefaultDNSRule
		conditions.ClashMode = ""
		conditions.Invert = false
		return evaluateClashMode(rule.DefaultOptions.ClashMode, mode, rule.DefaultOptions.Invert, !reflect.DeepEqual(conditions, option.RawDefaultDNSRule{}))
	case C.RuleTypeLogical:
		matches := make([]staticMatch, 0, len(rule.LogicalOptions.Rules))
		for _, subRule := range rule.LogicalOptions.Rules {
			matches = append(matches, evaluateDNSRule(subRule, mode))
		}
		return evaluateLogical(matches, rule.LogicalOptions.Mode, rule.LogicalOptions.Invert)
	default:
		return staticMatchUnknown
	}
}

func collectRuleReferences(rules []option.Rule, mode string, outbounds *[]string, transports *[]string) (shadowed bool) {
	for _, rule := range rules {
		match := evaluateRule(rule, mode)
		if match == staticMatchNever {
			continue
		}
		var action option.RuleAction
		switch rule.Type {
		case C.RuleTypeDefault:
			action = rule.DefaultOptions.RuleAction
		case C.RuleTypeLogical:
			action = rule.LogicalOptions.RuleAction
		}
		var final bool
		switch action.Action {
		case C.RuleActionTypeRoute:
			*outbounds = append(*outbounds, action.RouteOptions.Outbound)
			final = true
		case C.RuleActionTypeBypass:
			if action.BypassOptions.Outbound != "" {
				*outbounds = append(*outbounds, action.BypassOptions.Outbound)
				final = true
			}
		case C.RuleActionTypeDirect, C.RuleActionTypeReject, C.RuleActionTypeHijackDNS:
			final = true
		case C.RuleActionTypeResolve:
			*transports = append(*transports, action.ResolveOptions.Server)
		}
		if final && match == staticMatchAlways {
			return true
		}
	}
	return false
}

func collectDNSRuleReferences(rules []option.DNSRule, mode string, transports *[]string) (shadowed bool) {
	for _, rule := range rules {
		match := evaluateDNSRule(rule, mode)
		if match == staticMatchNever {
			continue
		}
		var action option.DNSRuleAction
		switch rule.Type {
		case C.RuleTypeDefault:
			action = rule.DefaultOptions.DNSRuleAction
		case C.RuleTypeLogical:
			action = rule.LogicalOptions.DNSRuleAction
		}
		var final bool
		switch action.Action {
		case C.RuleActionTypeRoute:
			*transports = append(*transports, action.RouteOptions.Server)
			final = true
		case C.RuleActionTypeEvaluate:
			*transports = append(*transports, action.EvaluateOptions.Server)
		case C.RuleActionTypeRespond, C.RuleActionTypeReject, C.RuleActionTypePredefined:
			final = true
		}
		if final && match == staticMatchAlways {
			return true
		}
	}
	return false
}
