package option

import (
	"net/url"
	"path/filepath"
	"reflect"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/domain"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"

	"go4.org/netipx"
)

type _RuleSet struct {
	Type          string        `json:"type,omitempty"`
	Tag           string        `json:"tag"`
	Format        string        `json:"format,omitempty"`
	InlineOptions PlainRuleSet  `json:"-"`
	LocalOptions  LocalRuleSet  `json:"-"`
	RemoteOptions RemoteRuleSet `json:"-"`
}

type RuleSet _RuleSet

func (r RuleSet) MarshalJSON() ([]byte, error) {
	if r.Type != C.RuleSetTypeInline {
		var defaultFormat string
		switch r.Type {
		case C.RuleSetTypeLocal:
			defaultFormat = ruleSetDefaultFormat(r.LocalOptions.Path)
		case C.RuleSetTypeRemote:
			defaultFormat = ruleSetDefaultFormat(r.RemoteOptions.URL)
		}
		if r.Format == defaultFormat {
			r.Format = ""
		}
	}
	var v any
	switch r.Type {
	case "", C.RuleSetTypeInline:
		r.Type = ""
		v = r.InlineOptions
	case C.RuleSetTypeLocal:
		v = r.LocalOptions
	case C.RuleSetTypeRemote:
		v = r.RemoteOptions
	default:
		return nil, E.New("unknown rule-set type: " + r.Type)
	}
	return badjson.MarshallObjects((_RuleSet)(r), v)
}

func (r *RuleSet) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_RuleSet)(r))
	if err != nil {
		return err
	}
	if r.Tag == "" {
		return E.New("missing tag")
	}
	var v any
	switch r.Type {
	case "", C.RuleSetTypeInline:
		r.Type = C.RuleSetTypeInline
		v = &r.InlineOptions
	case C.RuleSetTypeLocal:
		v = &r.LocalOptions
	case C.RuleSetTypeRemote:
		v = &r.RemoteOptions
	default:
		return E.New("unknown rule-set type: " + r.Type)
	}
	err = badjson.UnmarshallExcluded(bytes, (*_RuleSet)(r), v)
	if err != nil {
		return err
	}
	if r.Type != C.RuleSetTypeInline {
		if r.Format == "" {
			switch r.Type {
			case C.RuleSetTypeLocal:
				r.Format = ruleSetDefaultFormat(r.LocalOptions.Path)
			case C.RuleSetTypeRemote:
				r.Format = ruleSetDefaultFormat(r.RemoteOptions.URL)
			}
		}
		switch r.Format {
		case "":
			return E.New("missing format")
		case C.RuleSetFormatSource, C.RuleSetFormatBinary:
		default:
			return E.New("unknown rule-set format: " + r.Format)
		}
	} else {
		r.Format = ""
	}
	return nil
}

func ruleSetDefaultFormat(path string) string {
	if pathURL, err := url.Parse(path); err == nil {
		path = pathURL.Path
	}
	switch filepath.Ext(path) {
	case ".json":
		return C.RuleSetFormatSource
	case ".srs":
		return C.RuleSetFormatBinary
	default:
		return ""
	}
}

type LocalRuleSet struct {
	Path string `json:"path,omitempty"`
}

type RemoteRuleSet struct {
	URL            string             `json:"url"`
	DownloadDetour string             `json:"download_detour,omitempty"`
	UpdateInterval badoption.Duration `json:"update_interval,omitempty"`
}

type _HeadlessRule struct {
	Type           string              `json:"type,omitempty"`
	DefaultOptions DefaultHeadlessRule `json:"-"`
	LogicalOptions LogicalHeadlessRule `json:"-"`
}

type HeadlessRule _HeadlessRule

func (r HeadlessRule) MarshalJSON() ([]byte, error) {
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
	return badjson.MarshallObjects((_HeadlessRule)(r), v)
}

func (r *HeadlessRule) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_HeadlessRule)(r))
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
	err = badjson.UnmarshallExcluded(bytes, (*_HeadlessRule)(r), v)
	if err != nil {
		return err
	}
	return nil
}

func (r HeadlessRule) IsValid() bool {
	switch r.Type {
	case C.RuleTypeDefault, "":
		return r.DefaultOptions.IsValid()
	case C.RuleTypeLogical:
		return r.LogicalOptions.IsValid()
	default:
		panic("unknown rule type: " + r.Type)
	}
}

type DefaultHeadlessRule struct {
	QueryType               badoption.Listable[DNSQueryType]                                            `json:"query_type,omitempty"`
	Network                 badoption.Listable[string]                                                  `json:"network,omitempty"`
	Domain                  badoption.Listable[string]                                                  `json:"domain,omitempty"`
	DomainSuffix            badoption.Listable[string]                                                  `json:"domain_suffix,omitempty"`
	DomainKeyword           badoption.Listable[string]                                                  `json:"domain_keyword,omitempty"`
	DomainRegex             badoption.Listable[string]                                                  `json:"domain_regex,omitempty"`
	SourceIPCIDR            badoption.Listable[string]                                                  `json:"source_ip_cidr,omitempty"`
	IPCIDR                  badoption.Listable[string]                                                  `json:"ip_cidr,omitempty"`
	SourcePort              badoption.Listable[uint16]                                                  `json:"source_port,omitempty"`
	SourcePortRange         badoption.Listable[string]                                                  `json:"source_port_range,omitempty"`
	Port                    badoption.Listable[uint16]                                                  `json:"port,omitempty"`
	PortRange               badoption.Listable[string]                                                  `json:"port_range,omitempty"`
	ProcessName             badoption.Listable[string]                                                  `json:"process_name,omitempty"`
	ProcessPath             badoption.Listable[string]                                                  `json:"process_path,omitempty"`
	ProcessPathRegex        badoption.Listable[string]                                                  `json:"process_path_regex,omitempty"`
	PackageName             badoption.Listable[string]                                                  `json:"package_name,omitempty"`
	NetworkType             badoption.Listable[InterfaceType]                                           `json:"network_type,omitempty"`
	NetworkIsExpensive      bool                                                                        `json:"network_is_expensive,omitempty"`
	NetworkIsConstrained    bool                                                                        `json:"network_is_constrained,omitempty"`
	WIFISSID                badoption.Listable[string]                                                  `json:"wifi_ssid,omitempty"`
	WIFIBSSID               badoption.Listable[string]                                                  `json:"wifi_bssid,omitempty"`
	NetworkInterfaceAddress *badjson.TypedMap[InterfaceType, badoption.Listable[*badoption.Prefixable]] `json:"network_interface_address,omitempty"`
	DefaultInterfaceAddress badoption.Listable[*badoption.Prefixable]                                   `json:"default_interface_address,omitempty"`

	Invert bool `json:"invert,omitempty"`

	DomainMatcher *domain.Matcher `json:"-"`
	SourceIPSet   *netipx.IPSet   `json:"-"`
	IPSet         *netipx.IPSet   `json:"-"`

	AdGuardDomain        badoption.Listable[string] `json:"-"`
	AdGuardDomainMatcher *domain.AdGuardMatcher     `json:"-"`
}

func (r DefaultHeadlessRule) IsValid() bool {
	var defaultValue DefaultHeadlessRule
	defaultValue.Invert = r.Invert
	return !reflect.DeepEqual(r, defaultValue)
}

type LogicalHeadlessRule struct {
	Mode   string         `json:"mode"`
	Rules  []HeadlessRule `json:"rules,omitempty"`
	Invert bool           `json:"invert,omitempty"`
}

func (r LogicalHeadlessRule) IsValid() bool {
	return len(r.Rules) > 0 && common.All(r.Rules, HeadlessRule.IsValid)
}

type _PlainRuleSetCompat struct {
	Version    uint8           `json:"version"`
	Options    PlainRuleSet    `json:"-"`
	RawMessage json.RawMessage `json:"-"`
}

type PlainRuleSetCompat _PlainRuleSetCompat

func (r PlainRuleSetCompat) MarshalJSON() ([]byte, error) {
	var v any
	switch r.Version {
	case C.RuleSetVersion1, C.RuleSetVersion2, C.RuleSetVersion3, C.RuleSetVersion4:
		v = r.Options
	default:
		return nil, E.New("unknown rule-set version: ", r.Version)
	}
	return badjson.MarshallObjects((_PlainRuleSetCompat)(r), v)
}

func (r *PlainRuleSetCompat) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_PlainRuleSetCompat)(r))
	if err != nil {
		return err
	}
	var v any
	switch r.Version {
	case C.RuleSetVersion1, C.RuleSetVersion2, C.RuleSetVersion3, C.RuleSetVersion4:
		v = &r.Options
	case 0:
		return E.New("missing rule-set version")
	default:
		return E.New("unknown rule-set version: ", r.Version)
	}
	err = badjson.UnmarshallExcluded(bytes, (*_PlainRuleSetCompat)(r), v)
	if err != nil {
		return err
	}
	r.RawMessage = bytes
	return nil
}

func (r PlainRuleSetCompat) Upgrade() (PlainRuleSet, error) {
	switch r.Version {
	case C.RuleSetVersion1, C.RuleSetVersion2, C.RuleSetVersion3, C.RuleSetVersion4:
	default:
		return PlainRuleSet{}, E.New("unknown rule-set version: " + F.ToString(r.Version))
	}
	return r.Options, nil
}

type PlainRuleSet struct {
	Rules []HeadlessRule `json:"rules,omitempty"`
}
