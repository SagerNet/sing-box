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
		Parse: func(input string) (Link, error) {
			return ParseShadowSocks(input)
		},
	}))
}

// ParseShadowSocks parses official vemss link to Link
func ParseShadowSocks(ss string) (*SSLink, error) {
	url, err := url.Parse(ss)
	if err != nil {
		return nil, err
	}
	if url.Scheme != "ss" {
		return nil, E.New("not a ss:// link")
	}
	port, err := strconv.ParseUint(url.Port(), 10, 16)
	if err != nil {
		return nil, E.Cause(err, "invalid port")
	}
	link := &SSLink{
		OrigLink: ss,
		Address:  url.Hostname(),
		Port:     uint16(port),
		Ps:       url.Fragment,
	}
	queries := url.Query()
	for key, values := range queries {
		switch key {
		default:
			return nil, fmt.Errorf("unsupported shadowsocks parameter: %s=%v", key, values)
		}
	}
	if uname := url.User.Username(); uname != "" {
		if pass, ok := url.User.Password(); ok {
			link.Method = uname
			link.Password = pass
		} else {
			dec, err := base64Decode(uname)
			if err != nil {
				return nil, err
			}
			parts := strings.Split(string(dec), ":")
			link.Method = parts[0]
			if len(parts) > 1 {
				link.Password = parts[1]
			}
		}
	}
	return link, nil
}

// SSLink represents a parsed shadowsocks link
type SSLink struct {
	Method   string `json:"method,omitempty"`
	Password string `json:"password,omitempty"`
	Address  string `json:"address,omitempty"`
	Port     uint16 `json:"port,omitempty"`
	Ps       string `json:"ps,omitempty"`

	OrigLink string `json:"-,omitempty"`
}

// String implements Link
func (v SSLink) String() string {
	return v.OrigLink
}

// Options implements Link
func (v *SSLink) Options() *option.Outbound {
	return &option.Outbound{
		Type: "shadowsocks",
		Tag:  v.Ps,
		ShadowsocksOptions: option.ShadowsocksOutboundOptions{
			ServerOptions: option.ServerOptions{
				Server:     v.Address,
				ServerPort: v.Port,
			},
			Method:   v.Method,
			Password: v.Password,
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
