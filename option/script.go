package option

import (
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
)

type _ScriptSourceOptions struct {
	Source        string             `json:"source"`
	LocalOptions  LocalScriptSource  `json:"-"`
	RemoteOptions RemoteScriptSource `json:"-"`
}

type LocalScriptSource struct {
	Path string `json:"path"`
}

type RemoteScriptSource struct {
	URL            string             `json:"url"`
	DownloadDetour string             `json:"download_detour,omitempty"`
	UpdateInterval badoption.Duration `json:"update_interval,omitempty"`
}

type ScriptSourceOptions _ScriptSourceOptions

func (o ScriptSourceOptions) MarshalJSON() ([]byte, error) {
	var source any
	switch o.Source {
	case C.ScriptSourceLocal:
		source = o.LocalOptions
	case C.ScriptSourceRemote:
		source = o.RemoteOptions
	default:
		return nil, E.New("unknown script source: ", o.Source)
	}
	return badjson.MarshallObjects((_ScriptSourceOptions)(o), source)
}

func (o *ScriptSourceOptions) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_ScriptSourceOptions)(o))
	if err != nil {
		return err
	}
	var source any
	switch o.Source {
	case C.ScriptSourceLocal:
		source = &o.LocalOptions
	case C.ScriptSourceRemote:
		source = &o.RemoteOptions
	default:
		return E.New("unknown script source: ", o.Source)
	}
	return json.Unmarshal(bytes, source)
}

// TODO: make struct in order
type Script struct {
	ScriptSourceOptions
	ScriptOptions
}

func (s Script) MarshalJSON() ([]byte, error) {
	return badjson.MarshallObjects(s.ScriptSourceOptions, s.ScriptOptions)
}

func (s *Script) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, &s.ScriptSourceOptions)
	if err != nil {
		return err
	}
	return badjson.UnmarshallExcluded(bytes, &s.ScriptSourceOptions, &s.ScriptOptions)
}

type _ScriptOptions struct {
	Type        string             `json:"type"`
	Tag         string             `json:"tag"`
	Timeout     badoption.Duration `json:"timeout,omitempty"`
	Arguments   []any              `json:"arguments,omitempty"`
	HTTPOptions HTTPScriptOptions  `json:"-"`
	CronOptions CronScriptOptions  `json:"-"`
}

type ScriptOptions _ScriptOptions

func (o ScriptOptions) MarshalJSON() ([]byte, error) {
	var v any
	switch o.Type {
	case C.ScriptTypeSurgeGeneric:
		v = nil
	case C.ScriptTypeSurgeHTTPRequest, C.ScriptTypeSurgeHTTPResponse:
		v = o.HTTPOptions
	case C.ScriptTypeSurgeCron:
		v = o.CronOptions
	default:
		return nil, E.New("unknown script type: ", o.Type)
	}
	if v == nil {
		return badjson.MarshallObjects((_ScriptOptions)(o))
	}
	return badjson.MarshallObjects((_ScriptOptions)(o), v)
}

func (o *ScriptOptions) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_ScriptOptions)(o))
	if err != nil {
		return err
	}
	var v any
	switch o.Type {
	case C.ScriptTypeSurgeGeneric:
		v = nil
	case C.ScriptTypeSurgeHTTPRequest, C.ScriptTypeSurgeHTTPResponse:
		v = &o.HTTPOptions
	case C.ScriptTypeSurgeCron:
		v = &o.CronOptions
	default:
		return E.New("unknown script type: ", o.Type)
	}
	if v == nil {
		// check unknown fields
		return json.UnmarshalDisallowUnknownFields(bytes, &_ScriptOptions{})
	}
	return badjson.UnmarshallExcluded(bytes, (*_ScriptOptions)(o), v)
}

type HTTPScriptOptions struct {
	Pattern        string `json:"pattern"`
	RequiresBody   bool   `json:"requires_body,omitempty"`
	MaxSize        int64  `json:"max_size,omitempty"`
	BinaryBodyMode bool   `json:"binary_body_mode,omitempty"`
}

type CronScriptOptions struct {
	Expression string `json:"expression"`
}
