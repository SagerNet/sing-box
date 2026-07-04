package option

import (
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
)

type _SnellInboundOptions struct {
	ListenOptions
	Version     int                    `json:"version"`
	PSK         string                 `json:"psk"`
	Users       []SnellUser            `json:"users,omitempty"`
	ObfsOptions SnellObfsServerOptions `json:"-"`
	V6Options   SnellV6Options         `json:"-"`
}

type SnellInboundOptions _SnellInboundOptions

func (o *SnellInboundOptions) UnmarshalJSON(content []byte) error {
	err := json.Unmarshal(content, (*_SnellInboundOptions)(o))
	if err != nil {
		return err
	}
	var versionOptions any
	switch o.Version {
	case 5:
		versionOptions = &o.ObfsOptions
	case 6:
		versionOptions = &o.V6Options
	case 0:
		return E.New("snell: missing version")
	default:
		return E.New("snell: unsupported version: ", o.Version)
	}
	return badjson.UnmarshallExcluded(content, (*_SnellInboundOptions)(o), versionOptions)
}

func (o SnellInboundOptions) MarshalJSON() ([]byte, error) {
	var versionOptions any
	switch o.Version {
	case 5:
		versionOptions = o.ObfsOptions
	case 6:
		versionOptions = o.V6Options
	case 0:
		return nil, E.New("snell: missing version")
	default:
		return nil, E.New("snell: unsupported version: ", o.Version)
	}
	return badjson.MarshallObjects((_SnellInboundOptions)(o), versionOptions)
}

type _SnellOutboundOptions struct {
	DialerOptions
	ServerOptions
	Version     int                    `json:"version"`
	PSK         string                 `json:"psk"`
	UserKey     string                 `json:"userkey,omitempty"`
	Reuse       bool                   `json:"reuse,omitempty"`
	Network     NetworkList            `json:"network,omitempty"`
	ObfsOptions SnellObfsClientOptions `json:"-"`
	V6Options   SnellV6Options         `json:"-"`
}

type SnellOutboundOptions _SnellOutboundOptions

func (o *SnellOutboundOptions) UnmarshalJSON(content []byte) error {
	err := json.Unmarshal(content, (*_SnellOutboundOptions)(o))
	if err != nil {
		return err
	}
	var versionOptions any
	switch o.Version {
	case 4:
		versionOptions = &o.ObfsOptions
	case 6:
		versionOptions = &o.V6Options
	case 0:
		return E.New("snell: missing version")
	default:
		return E.New("snell: unsupported version: ", o.Version)
	}
	return badjson.UnmarshallExcluded(content, (*_SnellOutboundOptions)(o), versionOptions)
}

func (o SnellOutboundOptions) MarshalJSON() ([]byte, error) {
	var versionOptions any
	switch o.Version {
	case 4:
		versionOptions = o.ObfsOptions
	case 6:
		versionOptions = o.V6Options
	case 0:
		return nil, E.New("snell: missing version")
	default:
		return nil, E.New("snell: unsupported version: ", o.Version)
	}
	return badjson.MarshallObjects((_SnellOutboundOptions)(o), versionOptions)
}

type SnellObfsServerOptions struct {
	ObfsMode string `json:"obfs_mode,omitempty"`
}

type SnellUser struct {
	Name    string `json:"name,omitempty"`
	UserKey string `json:"userkey"`
}

type SnellObfsClientOptions struct {
	ObfsMode string `json:"obfs_mode,omitempty"`
	ObfsHost string `json:"obfs_host,omitempty"`
}

type SnellV6Options struct {
	Mode string `json:"mode,omitempty"`
}
