package option

import (
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badoption"
)

type _CertificateOptions struct {
	Store                    string                     `json:"store,omitempty"`
	Certificate              badoption.Listable[string] `json:"certificate,omitempty"`
	CertificatePath          badoption.Listable[string] `json:"certificate_path,omitempty"`
	CertificateDirectoryPath badoption.Listable[string] `json:"certificate_directory_path,omitempty"`
	TLSDecryption            *TLSDecryptionOptions      `json:"tls_decryption,omitempty"`
}

type TLSDecryptionOptions struct {
	Enabled         bool   `json:"enabled,omitempty"`
	KeyPair         string `json:"key_pair_p12,omitempty"`
	KeyPairPassword string `json:"key_pair_p12_password,omitempty"`
}

type CertificateOptions _CertificateOptions

func (o CertificateOptions) MarshalJSON() ([]byte, error) {
	switch o.Store {
	case C.CertificateStoreSystem:
		o.Store = ""
	}
	return json.Marshal((*_CertificateOptions)(&o))
}

func (o *CertificateOptions) UnmarshalJSON(data []byte) error {
	err := json.Unmarshal(data, (*_CertificateOptions)(o))
	if err != nil {
		return err
	}
	switch o.Store {
	case C.CertificateStoreSystem, "":
		o.Store = C.CertificateStoreSystem
	}
	return nil
}
