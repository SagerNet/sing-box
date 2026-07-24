package option

import (
	"reflect"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/schema"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badoption"
)

type _CertificateOptions struct {
	Store                    string                     `json:"store,omitempty" enum:"system,mozilla,chrome,none"`
	Certificate              badoption.Listable[string] `json:"certificate,omitempty"`
	CertificatePath          badoption.Listable[string] `json:"certificate_path,omitempty"`
	CertificateDirectoryPath badoption.Listable[string] `json:"certificate_directory_path,omitempty"`
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

func (o CertificateOptions) DescribeSchema(builder schema.Builder) (*schema.Node, error) {
	node := schema.StrictObject()
	err := builder.FlattenStruct(node, reflect.TypeFor[CertificateOptions]())
	if err != nil {
		return nil, err
	}
	return node, nil
}
