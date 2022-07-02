package option

import (
	"encoding/json"

	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
)

var ErrUnknownRuleType = E.New("unknown rule type")

type RouteOptions struct {
	GeoIP *GeoIPOptions `json:"geoip,omitempty"`
	Rules []Rule        `json:"rules,omitempty"`
}

type GeoIPOptions struct {
	Path           string `json:"path,omitempty"`
	DownloadURL    string `json:"download_url,omitempty"`
	DownloadDetour string `json:"download_detour,omitempty"`
}

type _Rule struct {
	Type           string       `json:"type,omitempty"`
	DefaultOptions *DefaultRule `json:"default_options,omitempty"`
	LogicalOptions *LogicalRule `json:"logical_options,omitempty"`
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
		if r.DefaultOptions == nil {
			break
		}
		err = json.Unmarshal(bytes, r.DefaultOptions)
	case C.RuleTypeLogical:
		if r.LogicalOptions == nil {
			break
		}
		err = json.Unmarshal(bytes, r.LogicalOptions)
	default:
		err = E.Extend(ErrUnknownRuleType, r.Type)
	}
	return err
}

type DefaultRule struct {
	Inbound       Listable[string] `json:"inbound,omitempty"`
	IPVersion     int              `json:"ip_version,omitempty"`
	Network       string           `json:"network,omitempty"`
	Protocol      Listable[string] `json:"protocol,omitempty"`
	Domain        Listable[string] `json:"domain,omitempty"`
	DomainSuffix  Listable[string] `json:"domain_suffix,omitempty"`
	DomainKeyword Listable[string] `json:"domain_keyword,omitempty"`
	SourceGeoIP   Listable[string] `json:"source_geoip,omitempty"`
	GeoIP         Listable[string] `json:"geoip,omitempty"`
	SourceIPCIDR  Listable[string] `json:"source_ip_cidr,omitempty"`
	IPCIDR        Listable[string] `json:"ip_cidr,omitempty"`
	SourcePort    Listable[uint16] `json:"source_port,omitempty"`
	Port          Listable[uint16] `json:"port,omitempty"`
	// ProcessName   Listable[string] `json:"process_name,omitempty"`
	// ProcessPath   Listable[string] `json:"process_path,omitempty"`
	Outbound string `json:"outbound,omitempty"`
}

type LogicalRule struct {
	Mode     string        `json:"mode"`
	Rules    []DefaultRule `json:"rules,omitempty"`
	Outbound string        `json:"outbound,omitempty"`
}
