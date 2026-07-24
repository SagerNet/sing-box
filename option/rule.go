package option

import (
	"context"
	"reflect"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/schema"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
)

type _Rule struct {
	Type           string      `json:"type,omitempty" enum:"default,logical"`
	DefaultOptions DefaultRule `json:"-"`
	LogicalOptions LogicalRule `json:"-"`
}

type Rule _Rule

func (r Rule) MarshalJSON() ([]byte, error) {
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
	return badjson.MarshallObjects((_Rule)(r), v)
}

func (r *Rule) UnmarshalJSONContext(ctx context.Context, bytes []byte) error {
	err := json.UnmarshalContext(ctx, bytes, (*_Rule)(r))
	if err != nil {
		return err
	}
	payload, err := rulePayloadWithoutType(ctx, bytes)
	if err != nil {
		return err
	}
	switch r.Type {
	case "", C.RuleTypeDefault:
		r.Type = C.RuleTypeDefault
		return unmarshalDefaultRuleContext(ctx, payload, &r.DefaultOptions)
	case C.RuleTypeLogical:
		return unmarshalLogicalRuleContext(ctx, payload, &r.LogicalOptions)
	default:
		return E.New("unknown rule type: " + r.Type)
	}
}

func (r Rule) IsValid() bool {
	switch r.Type {
	case C.RuleTypeDefault:
		return r.DefaultOptions.IsValid()
	case C.RuleTypeLogical:
		return r.LogicalOptions.IsValid()
	default:
		panic("unknown rule type: " + r.Type)
	}
}

func (r Rule) DescribeSchema(builder schema.Builder) (*schema.Node, error) {
	return builder.Define("Rule", func() (*schema.Node, error) {
		actionRef, err := builder.Define("RuleAction", func() (*schema.Node, error) {
			return routeActionUnion(builder)
		})
		if err != nil {
			return nil, err
		}
		nestedRef, err := builder.Define("NestedRule", func() (*schema.Node, error) {
			return nestedRuleUnion(builder, reflect.TypeFor[RawDefaultRule](), "NestedRule")
		})
		if err != nil {
			return nil, err
		}
		return ruleUnion(builder, reflect.TypeFor[RawDefaultRule](), nestedRef, actionRef)
	})
}

// ruleUnion builds the top-level rule schema: match fields composed with rule
// actions via unevaluatedProperties, mirroring the badjson.UnmarshallExcluded
// composition in DefaultRule / LogicalRule.
func ruleUnion(builder schema.Builder, matchType reflect.Type, nestedRef *schema.Node, actionRef *schema.Node) (*schema.Node, error) {
	defaultMatch := schema.LooseObject()
	defaultMatch.Properties.Put("type", schema.StringEnum(C.RuleTypeDefault, ""))
	err := builder.FlattenStruct(defaultMatch, matchType)
	if err != nil {
		return nil, err
	}
	defaultVariant := &schema.Node{
		Type:                  "object",
		AllOf:                 []*schema.Node{defaultMatch, actionRef},
		UnevaluatedProperties: false,
	}

	logicalMatch := schema.LooseObject()
	logicalMatch.Properties.Put("type", schema.StringConst(C.RuleTypeLogical))
	logicalProperties(logicalMatch, nestedRef)
	logicalMatch.Required = []string{"type", "mode", "rules"}
	logicalVariant := &schema.Node{
		Type:                  "object",
		AllOf:                 []*schema.Node{logicalMatch, actionRef},
		UnevaluatedProperties: false,
	}

	return schema.OneOf(defaultVariant, logicalVariant), nil
}

// nestedRuleUnion builds a match-only rule schema: nested rules reject rule
// actions, and headless rules never carry them.
func nestedRuleUnion(builder schema.Builder, matchType reflect.Type, selfName string) (*schema.Node, error) {
	defaultVariant := schema.StrictObject()
	defaultVariant.Properties.Put("type", schema.StringEnum(C.RuleTypeDefault, ""))
	err := builder.FlattenStruct(defaultVariant, matchType)
	if err != nil {
		return nil, err
	}

	logicalVariant := schema.StrictObject()
	logicalVariant.Properties.Put("type", schema.StringConst(C.RuleTypeLogical))
	logicalProperties(logicalVariant, schema.RefNode(selfName))
	logicalVariant.Required = []string{"type", "mode", "rules"}

	return schema.OneOf(defaultVariant, logicalVariant), nil
}

func logicalProperties(node *schema.Node, nestedRef *schema.Node) {
	node.Properties.Put("mode", schema.StringEnum(C.LogicalTypeAnd, C.LogicalTypeOr))
	node.Properties.Put("rules", &schema.Node{Type: "array", Items: nestedRef})
	node.Properties.Put("invert", schema.BooleanNode())
}

type RawDefaultRule struct {
	Inbound                  badoption.Listable[string]                                                  `json:"inbound,omitempty" reference:"inbound"`
	IPVersion                int                                                                         `json:"ip_version,omitempty" enum:"4,6"`
	Network                  badoption.Listable[string]                                                  `json:"network,omitempty" enum:"tcp,udp,icmp"`
	AuthUser                 badoption.Listable[string]                                                  `json:"auth_user,omitempty"`
	Protocol                 badoption.Listable[string]                                                  `json:"protocol,omitempty" enum:"tls,http,quic,dns,stun,bittorrent,dtls,ssh,rdp,ntp"`
	Client                   badoption.Listable[string]                                                  `json:"client,omitempty"`
	Domain                   badoption.Listable[string]                                                  `json:"domain,omitempty"`
	DomainSuffix             badoption.Listable[string]                                                  `json:"domain_suffix,omitempty"`
	DomainKeyword            badoption.Listable[string]                                                  `json:"domain_keyword,omitempty"`
	DomainRegex              badoption.Listable[string]                                                  `json:"domain_regex,omitempty"`
	Geosite                  badoption.Listable[string]                                                  `json:"geosite,omitempty" schema:"omit"`
	SourceGeoIP              badoption.Listable[string]                                                  `json:"source_geoip,omitempty" schema:"omit"`
	GeoIP                    badoption.Listable[string]                                                  `json:"geoip,omitempty" schema:"omit"`
	SourceIPCIDR             badoption.Listable[string]                                                  `json:"source_ip_cidr,omitempty"`
	SourceIPIsPrivate        bool                                                                        `json:"source_ip_is_private,omitempty"`
	IPCIDR                   badoption.Listable[string]                                                  `json:"ip_cidr,omitempty"`
	IPIsPrivate              bool                                                                        `json:"ip_is_private,omitempty"`
	SourcePort               badoption.Listable[uint16]                                                  `json:"source_port,omitempty"`
	SourcePortRange          badoption.Listable[string]                                                  `json:"source_port_range,omitempty"`
	Port                     badoption.Listable[uint16]                                                  `json:"port,omitempty"`
	PortRange                badoption.Listable[string]                                                  `json:"port_range,omitempty"`
	ProcessName              badoption.Listable[string]                                                  `json:"process_name,omitempty"`
	ProcessPath              badoption.Listable[string]                                                  `json:"process_path,omitempty"`
	ProcessPathRegex         badoption.Listable[string]                                                  `json:"process_path_regex,omitempty"`
	PackageName              badoption.Listable[string]                                                  `json:"package_name,omitempty"`
	PackageNameRegex         badoption.Listable[string]                                                  `json:"package_name_regex,omitempty"`
	User                     badoption.Listable[string]                                                  `json:"user,omitempty"`
	UserID                   badoption.Listable[int32]                                                   `json:"user_id,omitempty"`
	ClashMode                string                                                                      `json:"clash_mode,omitempty"`
	NetworkType              badoption.Listable[InterfaceType]                                           `json:"network_type,omitempty"`
	NetworkIsExpensive       bool                                                                        `json:"network_is_expensive,omitempty"`
	NetworkIsConstrained     bool                                                                        `json:"network_is_constrained,omitempty"`
	WIFISSID                 badoption.Listable[string]                                                  `json:"wifi_ssid,omitempty"`
	WIFIBSSID                badoption.Listable[string]                                                  `json:"wifi_bssid,omitempty"`
	InterfaceAddress         *badjson.TypedMap[string, badoption.Listable[*badoption.Prefixable]]        `json:"interface_address,omitempty"`
	NetworkInterfaceAddress  *badjson.TypedMap[InterfaceType, badoption.Listable[*badoption.Prefixable]] `json:"network_interface_address,omitempty"`
	DefaultInterfaceAddress  badoption.Listable[*badoption.Prefixable]                                   `json:"default_interface_address,omitempty"`
	SourceMACAddress         badoption.Listable[string]                                                  `json:"source_mac_address,omitempty"`
	SourceHostname           badoption.Listable[string]                                                  `json:"source_hostname,omitempty"`
	PreferredBy              badoption.Listable[string]                                                  `json:"preferred_by,omitempty"`
	RuleSet                  badoption.Listable[string]                                                  `json:"rule_set,omitempty" reference:"rule_set"`
	RuleSetIPCIDRMatchSource bool                                                                        `json:"rule_set_ip_cidr_match_source,omitempty"`
	Invert                   bool                                                                        `json:"invert,omitempty"`

	// Deprecated: renamed to rule_set_ip_cidr_match_source
	Deprecated_RulesetIPCIDRMatchSource bool `json:"rule_set_ipcidr_match_source,omitempty" schema:"omit"`
}

type DefaultRule struct {
	RawDefaultRule
	RuleAction
}

func (r DefaultRule) MarshalJSON() ([]byte, error) {
	return badjson.MarshallObjects(r.RawDefaultRule, r.RuleAction)
}

func (r *DefaultRule) UnmarshalJSON(data []byte) error {
	err := json.Unmarshal(data, &r.RawDefaultRule)
	if err != nil {
		return err
	}
	return badjson.UnmarshallExcluded(data, &r.RawDefaultRule, &r.RuleAction)
}

func (r DefaultRule) IsValid() bool {
	var defaultValue DefaultRule
	defaultValue.Invert = r.Invert
	return !reflect.DeepEqual(r, defaultValue)
}

type RawLogicalRule struct {
	Mode   string `json:"mode" enum:"and,or"`
	Rules  []Rule `json:"rules,omitempty"`
	Invert bool   `json:"invert,omitempty"`
}

type LogicalRule struct {
	RawLogicalRule
	RuleAction
}

func (r LogicalRule) MarshalJSON() ([]byte, error) {
	return badjson.MarshallObjects(r.RawLogicalRule, r.RuleAction)
}

func (r *LogicalRule) UnmarshalJSON(data []byte) error {
	err := json.Unmarshal(data, &r.RawLogicalRule)
	if err != nil {
		return err
	}
	return badjson.UnmarshallExcluded(data, &r.RawLogicalRule, &r.RuleAction)
}

func rulePayloadWithoutType(ctx context.Context, data []byte) ([]byte, error) {
	var content badjson.JSONObject
	err := content.UnmarshalJSONContext(ctx, data)
	if err != nil {
		return nil, err
	}
	content.Remove("type")
	return content.MarshalJSONContext(ctx)
}

func unmarshalDefaultRuleContext(ctx context.Context, data []byte, rule *DefaultRule) error {
	rawAction, routeOptions, err := inspectRouteRuleAction(ctx, data)
	if err != nil {
		return err
	}
	err = rejectNestedRouteRuleAction(ctx, data)
	if err != nil {
		return err
	}
	depth := nestedRuleDepth(ctx)
	err = json.UnmarshalContext(ctx, data, &rule.RawDefaultRule)
	if err != nil {
		return err
	}
	err = badjson.UnmarshallExcludedContext(ctx, data, &rule.RawDefaultRule, &rule.RuleAction)
	if err != nil {
		return err
	}
	if depth > 0 && rawAction == "" && routeOptions == (RouteActionOptions{}) {
		rule.RuleAction = RuleAction{}
	}
	return nil
}

func unmarshalLogicalRuleContext(ctx context.Context, data []byte, rule *LogicalRule) error {
	rawAction, routeOptions, err := inspectRouteRuleAction(ctx, data)
	if err != nil {
		return err
	}
	err = rejectNestedRouteRuleAction(ctx, data)
	if err != nil {
		return err
	}
	depth := nestedRuleDepth(ctx)
	err = json.UnmarshalContext(nestedRuleChildContext(ctx), data, &rule.RawLogicalRule)
	if err != nil {
		return err
	}
	err = badjson.UnmarshallExcludedContext(ctx, data, &rule.RawLogicalRule, &rule.RuleAction)
	if err != nil {
		return err
	}
	if depth > 0 && rawAction == "" && routeOptions == (RouteActionOptions{}) {
		rule.RuleAction = RuleAction{}
	}
	return nil
}

func (r *LogicalRule) IsValid() bool {
	return len(r.Rules) > 0 && common.All(r.Rules, Rule.IsValid)
}
