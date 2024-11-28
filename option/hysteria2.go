package option

import (
	"net/url"

	"github.com/sagernet/sing/common/json"
)

type Hysteria2InboundOptions struct {
	ListenOptions
	UpMbps                int             `json:"up_mbps,omitempty"`
	DownMbps              int             `json:"down_mbps,omitempty"`
	Obfs                  *Hysteria2Obfs  `json:"obfs,omitempty"`
	Users                 []Hysteria2User `json:"users,omitempty"`
	IgnoreClientBandwidth bool            `json:"ignore_client_bandwidth,omitempty"`
	InboundTLSOptionsContainer
	Masquerade  Hysteria2Masquerade `json:"masquerade,omitempty"`
	BrutalDebug bool                `json:"brutal_debug,omitempty"`
}

type Hysteria2Obfs struct {
	Type     string `json:"type,omitempty"`
	Password string `json:"password,omitempty"`
}

type Hysteria2User struct {
	Name     string `json:"name,omitempty"`
	Password string `json:"password,omitempty"`
}

type Hysteria2OutboundOptions struct {
	DialerOptions
	ServerOptions
	UpMbps   int            `json:"up_mbps,omitempty"`
	DownMbps int            `json:"down_mbps,omitempty"`
	Obfs     *Hysteria2Obfs `json:"obfs,omitempty"`
	Password string         `json:"password,omitempty"`
	Network  NetworkList    `json:"network,omitempty"`
	OutboundTLSOptionsContainer
	BrutalDebug bool `json:"brutal_debug,omitempty"`
}

type Hysteria2Masquerade struct {
	Type   string                   `json:"type,omitempty"`
	File   string                   `json:"file,omitempty"`
	Proxy  Hysteria2MasqueradeProxy `json:"proxy,omitempty"`
	String string                   `json:"string,omitempty"`
}

type Hysteria2MasqueradeProxy struct {
	URL         string `json:"url,omitempty"`
	RewriteHost bool   `json:"rewriteHost,omitempty"`
}

func (m *Hysteria2Masquerade) UnmarshalJSON(data []byte) error {
	// Attempt to unmarshal data as a string
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		masqueradeURL, err := url.Parse(str)
		if err != nil || masqueradeURL.Scheme == "" {
			m.String = str
			m.Type = "string"
			return nil
		}
		switch masqueradeURL.Scheme {
		case "file":
			m.File = masqueradeURL.Path
			m.Type = "file"
		case "http", "https":
			m.Proxy.URL = str
			m.Type = "proxy"
		default:
		}
		return nil
	}
	// If not a string, attempt to unmarshal into the struct
	type Alias Hysteria2Masquerade
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(m),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	return nil
}
