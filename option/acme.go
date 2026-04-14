package option

import (
	"strings"

	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
)

type ACMECertificateProviderOptions struct {
	Domain                  badoption.Listable[string]         `json:"domain,omitempty"`
	DataDirectory           string                             `json:"data_directory,omitempty"`
	DefaultServerName       string                             `json:"default_server_name,omitempty"`
	Email                   string                             `json:"email,omitempty"`
	Provider                string                             `json:"provider,omitempty"`
	AccountKey              string                             `json:"account_key,omitempty"`
	DisableHTTPChallenge    bool                               `json:"disable_http_challenge,omitempty"`
	DisableTLSALPNChallenge bool                               `json:"disable_tls_alpn_challenge,omitempty"`
	AlternativeHTTPPort     uint16                             `json:"alternative_http_port,omitempty"`
	AlternativeTLSPort      uint16                             `json:"alternative_tls_port,omitempty"`
	ExternalAccount         *ACMEExternalAccountOptions        `json:"external_account,omitempty"`
	DNS01Challenge          *ACMEProviderDNS01ChallengeOptions `json:"dns01_challenge,omitempty"`
	KeyType                 ACMEKeyType                        `json:"key_type,omitempty"`
	HTTPClient              *HTTPClientOptions                 `json:"http_client,omitempty"`
}

type _ACMEProviderDNS01ChallengeOptions struct {
	TTL                badoption.Duration         `json:"ttl,omitempty"`
	PropagationDelay   badoption.Duration         `json:"propagation_delay,omitempty"`
	PropagationTimeout badoption.Duration         `json:"propagation_timeout,omitempty"`
	Resolvers          badoption.Listable[string] `json:"resolvers,omitempty"`
	OverrideDomain     string                     `json:"override_domain,omitempty"`
	Provider           string                     `json:"provider,omitempty"`
	AliDNSOptions      ACMEDNS01AliDNSOptions     `json:"-"`
	CloudflareOptions  ACMEDNS01CloudflareOptions `json:"-"`
	ACMEDNSOptions     ACMEDNS01ACMEDNSOptions    `json:"-"`
}

type ACMEProviderDNS01ChallengeOptions _ACMEProviderDNS01ChallengeOptions

func (o ACMEProviderDNS01ChallengeOptions) MarshalJSON() ([]byte, error) {
	var v any
	switch o.Provider {
	case C.DNSProviderAliDNS:
		v = o.AliDNSOptions
	case C.DNSProviderCloudflare:
		v = o.CloudflareOptions
	case C.DNSProviderACMEDNS:
		v = o.ACMEDNSOptions
	case "":
		return nil, E.New("missing provider type")
	default:
		return nil, E.New("unknown provider type: ", o.Provider)
	}
	return badjson.MarshallObjects((_ACMEProviderDNS01ChallengeOptions)(o), v)
}

func (o *ACMEProviderDNS01ChallengeOptions) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_ACMEProviderDNS01ChallengeOptions)(o))
	if err != nil {
		return err
	}
	var v any
	switch o.Provider {
	case C.DNSProviderAliDNS:
		v = &o.AliDNSOptions
	case C.DNSProviderCloudflare:
		v = &o.CloudflareOptions
	case C.DNSProviderACMEDNS:
		v = &o.ACMEDNSOptions
	case "":
		return E.New("missing provider type")
	default:
		return E.New("unknown provider type: ", o.Provider)
	}
	return badjson.UnmarshallExcluded(bytes, (*_ACMEProviderDNS01ChallengeOptions)(o), v)
}

type ACMEKeyType string

const (
	ACMEKeyTypeED25519 = ACMEKeyType("ed25519")
	ACMEKeyTypeP256    = ACMEKeyType("p256")
	ACMEKeyTypeP384    = ACMEKeyType("p384")
	ACMEKeyTypeRSA2048 = ACMEKeyType("rsa2048")
	ACMEKeyTypeRSA4096 = ACMEKeyType("rsa4096")
)

func (t *ACMEKeyType) UnmarshalJSON(data []byte) error {
	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return err
	}
	value = strings.ToLower(value)
	switch ACMEKeyType(value) {
	case "", ACMEKeyTypeED25519, ACMEKeyTypeP256, ACMEKeyTypeP384, ACMEKeyTypeRSA2048, ACMEKeyTypeRSA4096:
		*t = ACMEKeyType(value)
	default:
		return E.New("unknown ACME key type: ", value)
	}
	return nil
}
