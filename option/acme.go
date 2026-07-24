package option

import (
	"reflect"
	"strings"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/schema"
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
	KeyType                 ACMEKeyType                        `json:"key_type,omitempty" enum:"ed25519,p256,p384,rsa2048,rsa4096"`
	Profile                 string                             `json:"profile,omitempty"`
	HTTPClient              *HTTPClientOptions                 `json:"http_client,omitempty"`
}

type _ACMEProviderDNS01ChallengeOptions struct {
	AbstractACMEProviderDNS01ChallengeOptions
	Provider          string                     `json:"provider,omitempty" enum:"alidns,cloudflare,acmedns"`
	AliDNSOptions     ACMEDNS01AliDNSOptions     `json:"-"`
	CloudflareOptions ACMEDNS01CloudflareOptions `json:"-"`
	ACMEDNSOptions    ACMEDNS01ACMEDNSOptions    `json:"-"`
}

type AbstractACMEProviderDNS01ChallengeOptions struct {
	TTL                badoption.Duration         `json:"ttl,omitempty"`
	PropagationDelay   badoption.Duration         `json:"propagation_delay,omitempty"`
	PropagationTimeout badoption.Duration         `json:"propagation_timeout,omitempty"`
	Resolvers          badoption.Listable[string] `json:"resolvers,omitempty"`
	OverrideDomain     string                     `json:"override_domain,omitempty"`
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

func (o ACMEProviderDNS01ChallengeOptions) DescribeSchema(builder schema.Builder) (*schema.Node, error) {
	return builder.Define("ACMEProviderDNS01Challenge", func() (*schema.Node, error) {
		return schema.DiscriminatedUnion(builder, "provider", true, acmeDNS01Variants(), func(variant *schema.Node) error {
			return builder.FlattenStruct(variant, reflect.TypeFor[AbstractACMEProviderDNS01ChallengeOptions]())
		})
	})
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

func (t ACMEKeyType) DescribeSchema(builder schema.Builder) (*schema.Node, error) {
	return schema.StringEnum("", "ed25519", "p256", "p384", "rsa2048", "rsa4096"), nil
}
