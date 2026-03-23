package option

import (
	"strings"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badoption"
)

type CloudflareOriginCACertificateProviderOptions struct {
	Domain            badoption.Listable[string]        `json:"domain,omitempty"`
	DataDirectory     string                            `json:"data_directory,omitempty"`
	APIToken          string                            `json:"api_token,omitempty"`
	OriginCAKey       string                            `json:"origin_ca_key,omitempty"`
	RequestType       CloudflareOriginCARequestType     `json:"request_type,omitempty"`
	RequestedValidity CloudflareOriginCARequestValidity `json:"requested_validity,omitempty"`
	Detour            string                            `json:"detour,omitempty"`
}

type CloudflareOriginCARequestType string

const (
	CloudflareOriginCARequestTypeOriginRSA = CloudflareOriginCARequestType("origin-rsa")
	CloudflareOriginCARequestTypeOriginECC = CloudflareOriginCARequestType("origin-ecc")
)

func (t *CloudflareOriginCARequestType) UnmarshalJSON(data []byte) error {
	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return err
	}
	value = strings.ToLower(value)
	switch CloudflareOriginCARequestType(value) {
	case "", CloudflareOriginCARequestTypeOriginRSA, CloudflareOriginCARequestTypeOriginECC:
		*t = CloudflareOriginCARequestType(value)
	default:
		return E.New("unsupported Cloudflare Origin CA request type: ", value)
	}
	return nil
}

type CloudflareOriginCARequestValidity uint16

const (
	CloudflareOriginCARequestValidity7    = CloudflareOriginCARequestValidity(7)
	CloudflareOriginCARequestValidity30   = CloudflareOriginCARequestValidity(30)
	CloudflareOriginCARequestValidity90   = CloudflareOriginCARequestValidity(90)
	CloudflareOriginCARequestValidity365  = CloudflareOriginCARequestValidity(365)
	CloudflareOriginCARequestValidity730  = CloudflareOriginCARequestValidity(730)
	CloudflareOriginCARequestValidity1095 = CloudflareOriginCARequestValidity(1095)
	CloudflareOriginCARequestValidity5475 = CloudflareOriginCARequestValidity(5475)
)

func (v *CloudflareOriginCARequestValidity) UnmarshalJSON(data []byte) error {
	var value uint16
	err := json.Unmarshal(data, &value)
	if err != nil {
		return err
	}
	switch CloudflareOriginCARequestValidity(value) {
	case 0,
		CloudflareOriginCARequestValidity7,
		CloudflareOriginCARequestValidity30,
		CloudflareOriginCARequestValidity90,
		CloudflareOriginCARequestValidity365,
		CloudflareOriginCARequestValidity730,
		CloudflareOriginCARequestValidity1095,
		CloudflareOriginCARequestValidity5475:
		*v = CloudflareOriginCARequestValidity(value)
	default:
		return E.New("unsupported Cloudflare Origin CA requested validity: ", value)
	}
	return nil
}
