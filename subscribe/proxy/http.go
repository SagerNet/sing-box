package proxy

import (
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"net"
	"net/url"
	"strconv"
)

type ProxyHTTP struct {
	Tag  string // 标签
	Type string // 代理类型
	//
	Dialer option.DialerOptions
	//
	Address   string // IP地址或域名
	Port      uint16 // 端口
	Username  string // 用户名
	Password  string // 密码
	TLSEnable bool   // 是否启用TLS
}

func (p *ProxyHTTP) GetTag() string {
	return p.Tag
}

func (p *ProxyHTTP) GetType() string {
	return C.TypeHTTP
}

func (p *ProxyHTTP) ParseLink(link string) error {
	u, err := url.Parse(link)
	if err != nil {
		return err
	}

	p.Address = u.Hostname()
	if u.Port() == "" {
		if u.Scheme == "https" {
			p.Port = 443
		} else {
			p.Port = 80
		}
	} else {
		portUint16, err := strconv.ParseUint(u.Port(), 10, 16)
		if err != nil {
			return err
		}
		p.Port = uint16(portUint16)
	}
	p.Username = u.User.Username()
	p.Password, _ = u.User.Password()

	p.Tag = u.Fragment

	if p.Tag == "" {
		p.Tag = net.JoinHostPort(p.Address, strconv.FormatUint(uint64(p.Port), 10))
	}

	if u.Scheme == "https" {
		p.TLSEnable = true
	}

	p.Type = C.TypeHTTP

	return nil
}

func (p *ProxyHTTP) SetDialer(dialer option.DialerOptions) {
	p.Dialer = dialer
}

func (p *ProxyHTTP) GenerateOutboundOptions() (option.Outbound, error) {
	out := option.Outbound{
		Tag:  p.Tag,
		Type: C.TypeHTTP,
		HTTPOptions: option.HTTPOutboundOptions{
			ServerOptions: option.ServerOptions{
				Server:     p.Address,
				ServerPort: p.Port,
			},
			Username: p.Username,
			Password: p.Password,
		},
	}

	if p.TLSEnable {
		out.HTTPOptions.TLS = &option.OutboundTLSOptions{
			Enabled: true,
		}
	}

	out.HTTPOptions.DialerOptions = p.Dialer

	return out, nil
}
