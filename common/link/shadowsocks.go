package link

import (
	"net/url"
	"strconv"
	"strings"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
)

var _ Link = (*ShadowSocks)(nil)

func init() {
	common.Must(RegisterParser(&Parser{
		Name:   "Shadowsocks",
		Scheme: []string{"ss"},
		Parse: func(u *url.URL) (Link, error) {
			link := &ShadowSocks{}
			return link, link.Parse(u)
		},
	}))
}

// ShadowSocks represents a parsed shadowsocks link
type ShadowSocks struct {
	Method     string `json:"method,omitempty"`
	Password   string `json:"password,omitempty"`
	Address    string `json:"address,omitempty"`
	Port       uint16 `json:"port,omitempty"`
	Ps         string `json:"ps,omitempty"`
	Plugin     string `json:"plugin,omitempty"`
	PluginOpts string `json:"plugin-opts,omitempty"`
}

// Options implements Link
func (l *ShadowSocks) Options() *option.Outbound {
	return &option.Outbound{
		Type: C.TypeShadowsocks,
		Tag:  l.Ps,
		ShadowsocksOptions: option.ShadowsocksOutboundOptions{
			ServerOptions: option.ServerOptions{
				Server:     l.Address,
				ServerPort: l.Port,
			},
			Method:        l.Method,
			Password:      l.Password,
			Plugin:        l.Plugin,
			PluginOptions: l.PluginOpts,
		},
	}
}

// Parse implements Link
//
// https://github.com/shadowsocks/shadowsocks-org/wiki/SIP002-URI-Scheme
func (l *ShadowSocks) Parse(u *url.URL) error {
	if u.Scheme != "ss" {
		return E.New("not a ss link")
	}
	port, err := strconv.ParseUint(u.Port(), 10, 16)
	if err != nil {
		return E.Cause(err, "invalid port")
	}
	l.Address = u.Hostname()
	l.Port = uint16(port)
	l.Ps = u.Fragment
	queries := u.Query()
	for key, values := range queries {
		switch key {
		case "plugin":
			parts := strings.SplitN(values[0], ";", 2)
			l.Plugin = parts[0]
			if len(parts) == 2 {
				l.PluginOpts = parts[1]
			}
		}
	}
	if uname := u.User.Username(); uname != "" {
		if pass, ok := u.User.Password(); ok {
			method, err := url.QueryUnescape(uname)
			if err != nil {
				return err
			}
			password, err := url.QueryUnescape(pass)
			if err != nil {
				return err
			}
			l.Method = method
			l.Password = password
		} else {
			dec, err := base64Decode(uname)
			if err != nil {
				return err
			}
			parts := strings.Split(string(dec), ":")
			l.Method = parts[0]
			if len(parts) > 1 {
				l.Password = parts[1]
			}
		}
	}
	return nil
}
