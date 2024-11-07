package option

import (
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
)

type InboundACMEOptions struct {
	Domain                  badoption.Listable[string]  `json:"domain,omitempty"`
	DataDirectory           string                      `json:"data_directory,omitempty"`
	DefaultServerName       string                      `json:"default_server_name,omitempty"`
	Email                   string                      `json:"email,omitempty"`
	Provider                string                      `json:"provider,omitempty"`
	DisableHTTPChallenge    bool                        `json:"disable_http_challenge,omitempty"`
	DisableTLSALPNChallenge bool                        `json:"disable_tls_alpn_challenge,omitempty"`
	AlternativeHTTPPort     uint16                      `json:"alternative_http_port,omitempty"`
	AlternativeTLSPort      uint16                      `json:"alternative_tls_port,omitempty"`
	ExternalAccount         *ACMEExternalAccountOptions `json:"external_account,omitempty"`
	DNS01Challenge          *ACMEDNS01ChallengeOptions  `json:"dns01_challenge,omitempty"`
}

type ACMEExternalAccountOptions struct {
	KeyID  string `json:"key_id,omitempty"`
	MACKey string `json:"mac_key,omitempty"`
}

type _ACMEDNS01ChallengeOptions struct {
	Provider          string                     `json:"provider,omitempty"`
	AliDNSOptions     ACMEDNS01AliDNSOptions     `json:"-"`
	CloudflareOptions ACMEDNS01CloudflareOptions `json:"-"`
}

type ACMEDNS01ChallengeOptions _ACMEDNS01ChallengeOptions

func (o ACMEDNS01ChallengeOptions) MarshalJSON() ([]byte, error) {
	var v any
	switch o.Provider {
	case C.DNSProviderAliDNS:
		v = o.AliDNSOptions
	case C.DNSProviderCloudflare:
		v = o.CloudflareOptions
	case "":
		return nil, E.New("missing provider type")
	default:
		return nil, E.New("unknown provider type: " + o.Provider)
	}
	return badjson.MarshallObjects((_ACMEDNS01ChallengeOptions)(o), v)
}

func (o *ACMEDNS01ChallengeOptions) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_ACMEDNS01ChallengeOptions)(o))
	if err != nil {
		return err
	}
	var v any
	switch o.Provider {
	case C.DNSProviderAliDNS:
		v = &o.AliDNSOptions
	case C.DNSProviderCloudflare:
		v = &o.CloudflareOptions
	default:
		return E.New("unknown provider type: " + o.Provider)
	}
	err = badjson.UnmarshallExcluded(bytes, (*_ACMEDNS01ChallengeOptions)(o), v)
	if err != nil {
		return err
	}
	return nil
}

type ACMEDNS01AliDNSOptions struct {
	AccessKeyID     string `json:"access_key_id,omitempty"`
	AccessKeySecret string `json:"access_key_secret,omitempty"`
	RegionID        string `json:"region_id,omitempty"`
}

type ACMEDNS01CloudflareOptions struct {
	APIToken string `json:"api_token,omitempty"`
}
