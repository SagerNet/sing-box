package option

import (
	"context"
	"reflect"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
)

type _DNSRule struct {
	Type           string         `json:"type,omitempty"`
	DefaultOptions DefaultDNSRule `json:"-"`
	LogicalOptions LogicalDNSRule `json:"-"`
}

type DNSRule _DNSRule

func (r DNSRule) MarshalJSON() ([]byte, error) {
	var v any
	switch r.Type {
	case C.RuleTypeDefault:
		r.Type = ""
		v = r.DefaultOptions
	case C.RuleTypeLogical:
		v = r.LogicalOptions
	default:
		return nil, E.New("unknown rule type: " + r.Type)
	}
	return badjson.MarshallObjects((_DNSRule)(r), v)
}

func (r *DNSRule) UnmarshalJSONContext(ctx context.Context, bytes []byte) error {
	err := json.Unmarshal(bytes, (*_DNSRule)(r))
	if err != nil {
		return err
	}
	var v any
	switch r.Type {
	case "", C.RuleTypeDefault:
		r.Type = C.RuleTypeDefault
		v = &r.DefaultOptions
	case C.RuleTypeLogical:
		v = &r.LogicalOptions
	default:
		return E.New("unknown rule type: " + r.Type)
	}
	err = badjson.UnmarshallExcludedContext(ctx, bytes, (*_DNSRule)(r), v)
	if err != nil {
		return err
	}
	return nil
}

func (r DNSRule) IsValid() bool {
	switch r.Type {
	case C.RuleTypeDefault:
		return r.DefaultOptions.IsValid()
	case C.RuleTypeLogical:
		return r.LogicalOptions.IsValid()
	default:
		panic("unknown DNS rule type: " + r.Type)
	}
}

// RawDefaultDNSRule已在dns.go中定义

type DefaultDNSRule struct {
	RawDefaultDNSRule
	DNSRuleAction
}

func (r DefaultDNSRule) MarshalJSON() ([]byte, error) {
	return badjson.MarshallObjects(r.RawDefaultDNSRule, r.DNSRuleAction)
}

func (r *DefaultDNSRule) UnmarshalJSONContext(ctx context.Context, data []byte) error {
	err := json.UnmarshalContext(ctx, data, &r.RawDefaultDNSRule)
	if err != nil {
		return err
	}
	return badjson.UnmarshallExcludedContext(ctx, data, &r.RawDefaultDNSRule, &r.DNSRuleAction)
}

func (r DefaultDNSRule) IsValid() bool {
	var defaultValue DefaultDNSRule
	defaultValue.Invert = r.Invert
	return !reflect.DeepEqual(r, defaultValue)
}

type RawLogicalDNSRule struct {
	Mode   string    `json:"mode"`
	Rules  []DNSRule `json:"rules,omitempty"`
	Invert bool      `json:"invert,omitempty"`
}

type LogicalDNSRule struct {
	RawLogicalDNSRule
	DNSRuleAction
}

func (r LogicalDNSRule) MarshalJSON() ([]byte, error) {
	return badjson.MarshallObjects(r.RawLogicalDNSRule, r.DNSRuleAction)
}

func (r *LogicalDNSRule) UnmarshalJSONContext(ctx context.Context, data []byte) error {
	err := json.Unmarshal(data, &r.RawLogicalDNSRule)
	if err != nil {
		return err
	}
	return badjson.UnmarshallExcludedContext(ctx, data, &r.RawLogicalDNSRule, &r.DNSRuleAction)
}

func (r *LogicalDNSRule) IsValid() bool {
	return len(r.Rules) > 0 && common.All(r.Rules, DNSRule.IsValid)
}
