package option

import (
	"reflect"

	"github.com/sagernet/sing-box/schema"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badoption"
)

type TailcatDERPOptions struct {
	DERPMapURL  string                                `json:"derp_map_url,omitempty"`
	DERPRegion  int                                   `json:"derp_region,omitempty"`
	DERPServers badoption.Listable[TailcatDERPServer] `json:"derp_servers,omitempty"`
	HTTPClient  *HTTPClientOptions                    `json:"http_client,omitempty"`
}

type _TailcatDERPServer struct {
	Host     string `json:"host"`
	IPv4     string `json:"ipv4,omitempty"`
	IPv6     string `json:"ipv6,omitempty"`
	DERPPort uint16 `json:"derp_port,omitempty"`
	STUNPort uint16 `json:"stun_port,omitempty"`
	CertName string `json:"cert_name,omitempty"`
}

type TailcatDERPServer _TailcatDERPServer

func (s TailcatDERPServer) MarshalJSON() ([]byte, error) {
	if s == (TailcatDERPServer{Host: s.Host}) {
		return json.Marshal(s.Host)
	}
	return json.Marshal(_TailcatDERPServer(s))
}

func (s *TailcatDERPServer) UnmarshalJSON(bytes []byte) error {
	var host string
	err := json.Unmarshal(bytes, &host)
	if err == nil {
		*s = TailcatDERPServer{Host: host}
		return nil
	}
	return json.UnmarshalDisallowUnknownFields(bytes, (*_TailcatDERPServer)(s))
}

func (s TailcatDERPServer) DescribeSchema(builder schema.Builder) (*schema.Node, error) {
	objectForm := schema.StrictObject()
	err := builder.FlattenStruct(objectForm, reflect.TypeFor[TailcatDERPServer]())
	if err != nil {
		return nil, err
	}
	return schema.AnyOf(schema.StringNode(), objectForm), nil
}

type TailcatUser struct {
	Name      string `json:"name,omitempty"`
	PublicKey string `json:"public_key"`
}

type TailcatInboundOptions struct {
	PrivateKey   string        `json:"private_key"`
	PreSharedKey string        `json:"pre_shared_key,omitempty"`
	Users        []TailcatUser `json:"users,omitempty"`
	TailcatDERPOptions
	DialerOptions
}

type TailcatOutboundOptions struct {
	PrivateKey      string `json:"private_key,omitempty"`
	ServerPublicKey string `json:"server_public_key"`
	ServerDiscoKey  string `json:"server_disco_key"`
	PreSharedKey    string `json:"pre_shared_key,omitempty"`
	TailcatDERPOptions
	DialerOptions
}
