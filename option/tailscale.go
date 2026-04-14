package option

import (
	"net/netip"
	"net/url"

	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
)

type TailscaleEndpointOptions struct {
	// Deprecated: use control_http_client instead
	DialerOptions
	StateDirectory             string                     `json:"state_directory,omitempty"`
	AuthKey                    string                     `json:"auth_key,omitempty"`
	ControlURL                 string                     `json:"control_url,omitempty"`
	ControlHTTPClient          *HTTPClientOptions         `json:"control_http_client,omitempty"`
	Ephemeral                  bool                       `json:"ephemeral,omitempty"`
	Hostname                   string                     `json:"hostname,omitempty"`
	AcceptRoutes               bool                       `json:"accept_routes,omitempty"`
	ExitNode                   string                     `json:"exit_node,omitempty"`
	ExitNodeAllowLANAccess     bool                       `json:"exit_node_allow_lan_access,omitempty"`
	AdvertiseRoutes            []netip.Prefix             `json:"advertise_routes,omitempty"`
	AdvertiseExitNode          bool                       `json:"advertise_exit_node,omitempty"`
	AdvertiseTags              badoption.Listable[string] `json:"advertise_tags,omitempty"`
	RelayServerPort            *uint16                    `json:"relay_server_port,omitempty"`
	RelayServerStaticEndpoints []netip.AddrPort           `json:"relay_server_static_endpoints,omitempty"`
	SystemInterface            bool                       `json:"system_interface,omitempty"`
	SystemInterfaceName        string                     `json:"system_interface_name,omitempty"`
	SystemInterfaceMTU         uint32                     `json:"system_interface_mtu,omitempty"`
	UDPTimeout                 UDPTimeoutCompat           `json:"udp_timeout,omitempty"`
}

type TailscaleDNSServerOptions struct {
	Endpoint               string `json:"endpoint,omitempty"`
	AcceptDefaultResolvers bool   `json:"accept_default_resolvers,omitempty"`
}

type TailscaleCertificateProviderOptions struct {
	Endpoint string `json:"endpoint,omitempty"`
}

type DERPServiceOptions struct {
	ListenOptions
	InboundTLSOptionsContainer
	ConfigPath           string                                          `json:"config_path,omitempty"`
	VerifyClientEndpoint badoption.Listable[string]                      `json:"verify_client_endpoint,omitempty"`
	VerifyClientURL      badoption.Listable[*DERPVerifyClientURLOptions] `json:"verify_client_url,omitempty"`
	Home                 string                                          `json:"home,omitempty"`
	MeshWith             badoption.Listable[*DERPMeshOptions]            `json:"mesh_with,omitempty"`
	MeshPSK              string                                          `json:"mesh_psk,omitempty"`
	MeshPSKFile          string                                          `json:"mesh_psk_file,omitempty"`
	STUN                 *DERPSTUNListenOptions                          `json:"stun,omitempty"`
}

type _DERPVerifyClientURLBase struct {
	URL string `json:"url,omitempty"`
}

type _DERPVerifyClientURLOptions struct {
	_DERPVerifyClientURLBase
	HTTPClientOptions
}

type DERPVerifyClientURLOptions _DERPVerifyClientURLOptions

func (d DERPVerifyClientURLOptions) ServerIsDomain() bool {
	verifyURL, err := url.Parse(d.URL)
	if err != nil {
		return false
	}
	return M.ParseSocksaddr(verifyURL.Hostname()).IsDomain()
}

func (d DERPVerifyClientURLOptions) MarshalJSON() ([]byte, error) {
	if d.URL != "" && d.HTTPClientOptions.IsEmpty() {
		return json.Marshal(d.URL)
	}
	return badjson.MarshallObjects(d._DERPVerifyClientURLBase, HTTPClient(d.HTTPClientOptions))
}

func (d *DERPVerifyClientURLOptions) UnmarshalJSON(bytes []byte) error {
	var stringValue string
	err := json.Unmarshal(bytes, &stringValue)
	if err == nil {
		*d = DERPVerifyClientURLOptions{
			_DERPVerifyClientURLBase: _DERPVerifyClientURLBase{URL: stringValue},
		}
		return nil
	}
	err = json.Unmarshal(bytes, &d._DERPVerifyClientURLBase)
	if err != nil {
		return err
	}
	var client HTTPClient
	err = badjson.UnmarshallExcluded(bytes, &d._DERPVerifyClientURLBase, &client)
	if err != nil {
		return err
	}
	d.HTTPClientOptions = HTTPClientOptions(client)
	return nil
}

type DERPMeshOptions struct {
	ServerOptions
	Host string `json:"host,omitempty"`
	OutboundTLSOptionsContainer
	DialerOptions
}

type _DERPSTUNListenOptions struct {
	Enabled bool
	ListenOptions
}

type DERPSTUNListenOptions _DERPSTUNListenOptions

func (d DERPSTUNListenOptions) MarshalJSON() ([]byte, error) {
	portOptions := _DERPSTUNListenOptions{
		Enabled: d.Enabled,
		ListenOptions: ListenOptions{
			ListenPort: d.ListenPort,
		},
	}
	if _DERPSTUNListenOptions(d) == portOptions {
		return json.Marshal(d.Enabled)
	} else {
		return json.Marshal(_DERPSTUNListenOptions(d))
	}
}

func (d *DERPSTUNListenOptions) UnmarshalJSON(bytes []byte) error {
	var portValue uint16
	err := json.Unmarshal(bytes, &portValue)
	if err == nil {
		d.Enabled = true
		d.ListenPort = portValue
		return nil
	}
	return json.Unmarshal(bytes, (*_DERPSTUNListenOptions)(d))
}
