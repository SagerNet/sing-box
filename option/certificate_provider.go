package option

import (
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
)

type _CertificateProviderOptions struct {
	Type        string                         `json:"type"`
	ACMEOptions CertificateProviderACMEOptions `json:"-"`
}

type CertificateProviderOptions _CertificateProviderOptions

func (o CertificateProviderOptions) MarshalJSON() ([]byte, error) {
	var v any
	switch o.Type {
	case C.TypeACME:
		v = o.ACMEOptions
	case "":
		return nil, E.New("missing certificate provider type")
	default:
		return nil, E.New("unknown certificate provider type: ", o.Type)
	}
	return badjson.MarshallObjects((_CertificateProviderOptions)(o), v)
}

func (o *CertificateProviderOptions) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_CertificateProviderOptions)(o))
	if err != nil {
		return err
	}
	var v any
	switch o.Type {
	case C.TypeACME:
		v = &o.ACMEOptions
	case "":
		return E.New("missing certificate provider type")
	default:
		return E.New("unknown certificate provider type: ", o.Type)
	}
	err = badjson.UnmarshallExcluded(bytes, (*_CertificateProviderOptions)(o), v)
	if err != nil {
		return err
	}
	return nil
}

type CertificateProviderACMEOptions struct {
	Service string `json:"service"`
}
