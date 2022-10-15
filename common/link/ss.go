package link

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
)

var _ Link = (*SSLink)(nil)

func init() {
	common.Must(RegisterParser(&Parser{
		Name:   "Shadowsocks",
		Scheme: []string{"ss"},
		Parse: func(u *url.URL) (Link, error) {
			link := &SSLink{}
			return link, link.Parse(u)
		},
	}))
}

// Parse implements Link
func (l *SSLink) Parse(u *url.URL) error {
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
		default:
			return fmt.Errorf("unsupported shadowsocks parameter: %s=%v", key, values)
		}
	}
	if uname := u.User.Username(); uname != "" {
		if pass, ok := u.User.Password(); ok {
			l.Method = uname
			l.Password = pass
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

// SSLink represents a parsed shadowsocks link
type SSLink struct {
	Method   string `json:"method,omitempty"`
	Password string `json:"password,omitempty"`
	Address  string `json:"address,omitempty"`
	Port     uint16 `json:"port,omitempty"`
	Ps       string `json:"ps,omitempty"`
}

// Options implements Link
func (l *SSLink) Options() *option.Outbound {
	return &option.Outbound{
		Type: "shadowsocks",
		Tag:  l.Ps,
		ShadowsocksOptions: option.ShadowsocksOutboundOptions{
			ServerOptions: option.ServerOptions{
				Server:     l.Address,
				ServerPort: l.Port,
			},
			Method:   l.Method,
			Password: l.Password,
		},
	}
}

func base64Decode(b64 string) ([]byte, error) {
	b64 = strings.TrimSpace(b64)
	stdb64 := b64
	if pad := len(b64) % 4; pad != 0 {
		stdb64 += strings.Repeat("=", 4-pad)
	}

	b, err := base64.StdEncoding.DecodeString(stdb64)
	if err != nil {
		return base64.URLEncoding.DecodeString(b64)
	}
	return b, nil
}
