package option

import (
	"github.com/sagernet/sing-box/common/json"
	E "github.com/sagernet/sing/common/exceptions"
	"regexp"
)

type SubscribeOutboundOptions struct {
	Url                  string                `json:"url"`
	CacheFile            string                `json:"cache_file,omitempty"`
	ForceUpdateDuration  Duration              `json:"force_update_duration,omitempty"`
	DNS                  string                `json:"dns,omitempty"`
	Filter               *Filter               `json:"filter,omitempty"`
	RequestDialerOptions *RequestDialerOptions `json:"request_dialer,omitempty"`
	DialerOptions        *DialerOptions        `json:"dialer,omitempty"`
	ProxyGroupOptions
	CustomGroup Listable[CustomGroupOptions] `json:"custom_group,omitempty"`
}

type ProxyGroupOptions struct {
	ProxyType       string                   `json:"proxy_type"`
	SelectorOptions *SelectorOutboundOptions `json:"selector,omitempty"`
	URLTestOptions  *URLTestOutboundOptions  `json:"urltest,omitempty"`
}

type CustomGroupOptions struct {
	Tag    string  `json:"tag,omitempty"`
	Filter *Filter `json:"filter,omitempty"`
	ProxyGroupOptions
}

type Filter struct {
	WhiteMode bool                     `json:"white_mode,omitempty"`
	Rule      Listable[*regexp.Regexp] `json:"rule,omitempty"`
}

type _filter struct {
	WhiteMode bool             `json:"white_mode,omitempty"`
	Rule      Listable[string] `json:"rule,omitempty"`
}

func (f *Filter) UnmarshalJSON(content []byte) error {
	var _f _filter
	err := json.Unmarshal(content, &_f)
	if err != nil {
		return err
	}
	f.WhiteMode = _f.WhiteMode
	f.Rule = make(Listable[*regexp.Regexp], 0)
	for _, r := range _f.Rule {
		reg, err := regexp.Compile(r)
		if err != nil {
			return E.New("invalid regexp: ", r)
		}
		f.Rule = append(f.Rule, reg)
	}
	return nil
}

func (f Filter) MarshalJSON() ([]byte, error) {
	_f := _filter{
		WhiteMode: f.WhiteMode,
		Rule:      make(Listable[string], 0),
	}
	for _, r := range f.Rule {
		_f.Rule = append(_f.Rule, r.String())
	}
	return json.Marshal(_f)
}

type RequestDialerOptions struct {
	BindInterface      string         `json:"bind_interface,omitempty"`
	Inet4BindAddress   *ListenAddress `json:"inet4_bind_address,omitempty"`
	Inet6BindAddress   *ListenAddress `json:"inet6_bind_address,omitempty"`
	ProtectPath        string         `json:"protect_path,omitempty"`
	RoutingMark        int            `json:"routing_mark,omitempty"`
	ReuseAddr          bool           `json:"reuse_addr,omitempty"`
	ConnectTimeout     Duration       `json:"connect_timeout,omitempty"`
	TCPFastOpen        bool           `json:"tcp_fast_open,omitempty"`
	UDPFragment        *bool          `json:"udp_fragment,omitempty"`
	UDPFragmentDefault bool           `json:"-"`
}
