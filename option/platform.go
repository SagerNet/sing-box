package option

import (
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badoption"
)

type OnDemandOptions struct {
	Enabled bool           `json:"enabled,omitempty"`
	Rules   []OnDemandRule `json:"rules,omitempty"`
}

type OnDemandRule struct {
	Action                *OnDemandRuleAction        `json:"action,omitempty"`
	DNSSearchDomainMatch  badoption.Listable[string] `json:"dns_search_domain_match,omitempty"`
	DNSServerAddressMatch badoption.Listable[string] `json:"dns_server_address_match,omitempty"`
	InterfaceTypeMatch    *OnDemandRuleInterfaceType `json:"interface_type_match,omitempty"`
	SSIDMatch             badoption.Listable[string] `json:"ssid_match,omitempty"`
	ProbeURL              string                     `json:"probe_url,omitempty"`
}

type OnDemandRuleAction int

func (r *OnDemandRuleAction) MarshalJSON() ([]byte, error) {
	if r == nil {
		return nil, nil
	}
	value := *r
	var actionName string
	switch value {
	case 1:
		actionName = "connect"
	case 2:
		actionName = "disconnect"
	case 3:
		actionName = "evaluate_connection"
	default:
		return nil, E.New("unknown action: ", value)
	}
	return json.Marshal(actionName)
}

func (r *OnDemandRuleAction) UnmarshalJSON(bytes []byte) error {
	var actionName string
	if err := json.Unmarshal(bytes, &actionName); err != nil {
		return err
	}
	var actionValue int
	switch actionName {
	case "connect":
		actionValue = 1
	case "disconnect":
		actionValue = 2
	case "evaluate_connection":
		actionValue = 3
	case "ignore":
		actionValue = 4
	default:
		return E.New("unknown action name: ", actionName)
	}
	*r = OnDemandRuleAction(actionValue)
	return nil
}

type OnDemandRuleInterfaceType int

func (r *OnDemandRuleInterfaceType) MarshalJSON() ([]byte, error) {
	if r == nil {
		return nil, nil
	}
	value := *r
	var interfaceTypeName string
	switch value {
	case 1:
		interfaceTypeName = "any"
	case 2:
		interfaceTypeName = "wifi"
	case 3:
		interfaceTypeName = "cellular"
	default:
		return nil, E.New("unknown interface type: ", value)
	}
	return json.Marshal(interfaceTypeName)
}

func (r *OnDemandRuleInterfaceType) UnmarshalJSON(bytes []byte) error {
	var interfaceTypeName string
	if err := json.Unmarshal(bytes, &interfaceTypeName); err != nil {
		return err
	}
	var interfaceTypeValue int
	switch interfaceTypeName {
	case "any":
		interfaceTypeValue = 1
	case "wifi":
		interfaceTypeValue = 2
	case "cellular":
		interfaceTypeValue = 3
	default:
		return E.New("unknown interface type name: ", interfaceTypeName)
	}
	*r = OnDemandRuleInterfaceType(interfaceTypeValue)
	return nil
}
