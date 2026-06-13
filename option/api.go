package option

import (
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badoption"
)

type APIServiceOptions struct {
	ListenOptions
	Secret                           string                     `json:"secret,omitempty"`
	AccessControlAllowOrigin         badoption.Listable[string] `json:"access_control_allow_origin,omitempty"`
	AccessControlAllowPrivateNetwork bool                       `json:"access_control_allow_private_network,omitempty"`
	Dashboard                        *APIDashboardOptions       `json:"dashboard,omitempty"`
	InboundTLSOptionsContainer
}

type _APIDashboardOptions struct {
	Enabled        bool               `json:"enabled,omitempty"`
	Path           string             `json:"path,omitempty"`
	DownloadURL    string             `json:"download_url,omitempty"`
	HTTPClient     *HTTPClientOptions `json:"http_client,omitempty"`
	UpdateInterval badoption.Duration `json:"update_interval,omitempty"`
}

type APIDashboardOptions _APIDashboardOptions

func (o APIDashboardOptions) MarshalJSON() ([]byte, error) {
	if o.DownloadURL == "" && o.HTTPClient == nil && o.UpdateInterval == 0 {
		if o.Path == "" {
			return json.Marshal(o.Enabled)
		}
		if o.Enabled {
			return json.Marshal(o.Path)
		}
	}
	return json.Marshal(_APIDashboardOptions(o))
}

func (o *APIDashboardOptions) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, &o.Enabled)
	if err == nil {
		return nil
	}
	err = json.Unmarshal(bytes, &o.Path)
	if err == nil {
		o.Enabled = true
		return nil
	}
	return json.UnmarshalDisallowUnknownFields(bytes, (*_APIDashboardOptions)(o))
}
