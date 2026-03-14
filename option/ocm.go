package option

import (
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
)

type OCMServiceOptions struct {
	ListenOptions
	InboundTLSOptionsContainer
	CredentialPath string               `json:"credential_path,omitempty"`
	Credentials    []OCMCredential      `json:"credentials,omitempty"`
	Users          []OCMUser            `json:"users,omitempty"`
	Headers        badoption.HTTPHeader `json:"headers,omitempty"`
	Detour         string               `json:"detour,omitempty"`
	UsagesPath     string               `json:"usages_path,omitempty"`
}

type OCMUser struct {
	Name               string `json:"name,omitempty"`
	Token              string `json:"token,omitempty"`
	Credential         string `json:"credential,omitempty"`
	ExternalCredential string `json:"external_credential,omitempty"`
	AllowExternalUsage bool   `json:"allow_external_usage,omitempty"`
}

type _OCMCredential struct {
	Type            string                       `json:"type,omitempty"`
	Tag             string                       `json:"tag"`
	DefaultOptions  OCMDefaultCredentialOptions  `json:"-"`
	ExternalOptions OCMExternalCredentialOptions `json:"-"`
	BalancerOptions OCMBalancerCredentialOptions `json:"-"`
	FallbackOptions OCMFallbackCredentialOptions `json:"-"`
}

type OCMCredential _OCMCredential

func (c OCMCredential) MarshalJSON() ([]byte, error) {
	var v any
	switch c.Type {
	case "", "default":
		c.Type = ""
		v = c.DefaultOptions
	case "external":
		v = c.ExternalOptions
	case "balancer":
		v = c.BalancerOptions
	case "fallback":
		v = c.FallbackOptions
	default:
		return nil, E.New("unknown credential type: ", c.Type)
	}
	return badjson.MarshallObjects((_OCMCredential)(c), v)
}

func (c *OCMCredential) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_OCMCredential)(c))
	if err != nil {
		return err
	}
	if c.Tag == "" {
		return E.New("missing credential tag")
	}
	var v any
	switch c.Type {
	case "", "default":
		c.Type = "default"
		v = &c.DefaultOptions
	case "external":
		v = &c.ExternalOptions
	case "balancer":
		v = &c.BalancerOptions
	case "fallback":
		v = &c.FallbackOptions
	default:
		return E.New("unknown credential type: ", c.Type)
	}
	return badjson.UnmarshallExcluded(bytes, (*_OCMCredential)(c), v)
}

type OCMDefaultCredentialOptions struct {
	CredentialPath string `json:"credential_path,omitempty"`
	UsagesPath     string `json:"usages_path,omitempty"`
	Detour         string `json:"detour,omitempty"`
	Reserve5h      uint8  `json:"reserve_5h"`
	ReserveWeekly  uint8  `json:"reserve_weekly"`
	Limit5h        uint8  `json:"limit_5h,omitempty"`
	LimitWeekly    uint8  `json:"limit_weekly,omitempty"`
}

type OCMBalancerCredentialOptions struct {
	Strategy     string                     `json:"strategy,omitempty"`
	Credentials  badoption.Listable[string] `json:"credentials"`
	PollInterval badoption.Duration         `json:"poll_interval,omitempty"`
}

type OCMExternalCredentialOptions struct {
	URL string `json:"url,omitempty"`
	ServerOptions
	Token        string             `json:"token"`
	Reverse      bool               `json:"reverse,omitempty"`
	Detour       string             `json:"detour,omitempty"`
	UsagesPath   string             `json:"usages_path,omitempty"`
	PollInterval badoption.Duration `json:"poll_interval,omitempty"`
}

type OCMFallbackCredentialOptions struct {
	Credentials  badoption.Listable[string] `json:"credentials"`
	PollInterval badoption.Duration         `json:"poll_interval,omitempty"`
}
