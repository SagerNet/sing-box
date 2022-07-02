package option

import (
	"encoding/json"

	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
)

var ErrUnknownRuleType = E.New("unknown rule type")

type _Rule struct {
	Type           string      `json:"type"`
	DefaultOptions DefaultRule `json:"default_options,omitempty"`
	LogicalOptions LogicalRule `json:"logical_options,omitempty"`
}

type Rule _Rule

func (r *Rule) MarshalJSON() ([]byte, error) {
	var content map[string]any
	switch r.Type {
	case "", C.RuleTypeDefault:
		return json.Marshal(r.DefaultOptions)
	case C.RuleTypeLogical:
		options, err := json.Marshal(r.LogicalOptions)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(options, &content)
		if err != nil {
			return nil, err
		}
		content["type"] = r.Type
		return json.Marshal(content)
	default:
		return nil, E.Extend(ErrUnknownRuleType, r.Type)
	}
}

func (r *Rule) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_Rule)(r))
	if err != nil {
		return err
	}
	switch r.Type {
	case "", C.RuleTypeDefault:
		err = json.Unmarshal(bytes, &r.DefaultOptions)
	case C.RuleTypeLogical:
		err = json.Unmarshal(bytes, &r.LogicalOptions)
	default:
		err = E.Extend(ErrUnknownRuleType, r.Type)
	}
	return err
}

type DefaultRule struct {
	Inbound       []string `json:"inbound,omitempty"`
	IPVersion     []int    `json:"ip_version,omitempty"`
	Network       []string `json:"network,omitempty"`
	Protocol      []string `json:"protocol,omitempty"`
	Domain        []string `json:"domain,omitempty"`
	DomainSuffix  []string `json:"domain_suffix,omitempty"`
	DomainKeyword []string `json:"domain_keyword,omitempty"`
	SourceGeoIP   []string `json:"source_geoip,omitempty"`
	GeoIP         []string `json:"geoip,omitempty"`
	SourceIPCIDR  []string `json:"source_ipcidr,omitempty"`
	SourcePort    []string `json:"source_port,omitempty"`
	IPCIDR        []string `json:"destination_ipcidr,omitempty"`
	Port          []string `json:"destination_port,omitempty"`
	ProcessName   []string `json:"process_name,omitempty"`
	ProcessPath   []string `json:"process_path,omitempty"`
	Outbound      string   `json:"outbound,omitempty"`
}

type LogicalRule struct {
	Mode     string        `json:"mode"`
	Rules    []DefaultRule `json:"rules,omitempty"`
	Outbound string        `json:"outbound,omitempty"`
}
