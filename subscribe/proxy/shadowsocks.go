package proxy

import (
	"encoding/base64"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"net"
	"net/url"
	"strconv"
	"strings"
)

type ProxyShadowsocks struct {
	Tag  string // 标签
	Type string // 代理类型
	//
	Dialer option.DialerOptions
	//
	Address       string // IP地址或域名
	Port          uint16 // 端口
	Method        string // 加密方式
	Password      string // 密码
	Plugin        string // 插件
	PluginOptions string // 插件选项
}

func (p *ProxyShadowsocks) GetType() string {
	return C.TypeSocks
}

func (p *ProxyShadowsocks) GetTag() string {
	return p.Tag
}

func (p *ProxyShadowsocks) ParseLink(link string) error {
	configStr := strings.TrimPrefix(link, "ss://")

	r := func(uri string) int {
		func() {
			var (
				suri = strings.Split(uri, "#")
				stag = ""
			)
			if len(suri) <= 2 {
				if len(suri) == 2 {
					stag = "#" + suri[1]
				}
				suriDecode, err := Base64Decode(suri[0])
				if err != nil {
					return
				}
				uri = string(suriDecode) + stag
			}
		}()
		u, err := url.Parse("http://" + uri)
		if err != nil {
			return 1
		}
		if u.Host == "" {
			return 1
		}
		host, port, err := net.SplitHostPort(u.Host)
		if err != nil {
			host = u.Hostname()
			port = "80"
		}
		portUint, err := strconv.ParseUint(port, 10, 16)
		if err != nil || portUint == 0 || portUint > 65535 {
			return 1
		}
		var userinfo []string
		if u.User != nil {
			username := u.User.Username()
			password, _ := u.User.Password()
			if username != "" && password != "" {
				userinfo = []string{username, password}
			} else if username != "" {
				usernameDecode, err := base64.RawURLEncoding.DecodeString(username)
				if err != nil {
					return 1
				}
				userinfo = strings.Split(string(usernameDecode), ":")
				if len(userinfo) != 2 {
					return 1
				}
			}
		} else {
			return 1
		}

		p.Address = host
		p.Port = uint16(portUint)
		p.Method = userinfo[0]
		p.Password = userinfo[1]

		if u.RawQuery != "" {
			plugin := u.Query().Get("plugin")
			if plugin != "" {
				pluginInfo := strings.Split(plugin, ";")
				pi := pluginInfo[0]
				p.Plugin = pi
				if pi != "" {
					pluginOpts := strings.Join(pluginInfo[1:], ";")
					p.PluginOptions = pluginOpts
				}
			}
		}

		if u.Fragment != "" {
			p.Tag = u.Fragment
		} else {
			p.Tag = net.JoinHostPort(host, port)
		}

		return 0

	}(configStr)

	if r != 0 {
		uriSlice := strings.Split(configStr, "@")
		if len(uriSlice) < 2 {
			return E.New("invalid shadowsocks link")
		} else if len(uriSlice) > 2 {
			uriSlice = []string{strings.Join(uriSlice[:len(uriSlice)-1], "@"), uriSlice[len(uriSlice)-1]}
		}
		host, port, err := net.SplitHostPort(uriSlice[1])
		if err != nil {
			return E.New("invalid shadowsocks link")
		}
		portUint, err := strconv.ParseUint(port, 10, 16)
		if err != nil || portUint == 0 || portUint > 65535 {
			return E.New("invalid shadowsocks link")
		}
		authInfo := strings.SplitN(uriSlice[0], ":", 2)

		p.Address = host
		p.Port = uint16(portUint)
		p.Method = authInfo[0]
		p.Password = authInfo[1]

		p.Tag = net.JoinHostPort(host, port)
	}

	p.Type = C.TypeShadowsocks

	return nil
}

func (p *ProxyShadowsocks) SetDialer(dialer option.DialerOptions) {
	p.Dialer = dialer
}

func (p *ProxyShadowsocks) GenerateOutboundOptions() (option.Outbound, error) {

	if !checkShadowsocksAllowMethod(p.Method) {
		return option.Outbound{}, E.New("shadowsocks method '", p.Method, "' is not supported in sing-box")
	}

	out := option.Outbound{
		Tag:  p.Tag,
		Type: C.TypeShadowsocks,
		ShadowsocksOptions: option.ShadowsocksOutboundOptions{
			ServerOptions: option.ServerOptions{
				Server:     p.Address,
				ServerPort: p.Port,
			},
			Password: p.Password,
			Method:   p.Method,
		},
	}

	if p.Plugin != "" {
		out.ShadowsocksOptions.Plugin = p.Plugin
		out.ShadowsocksOptions.PluginOptions = p.PluginOptions
	}

	out.ShadowsocksOptions.DialerOptions = p.Dialer

	return out, nil
}
