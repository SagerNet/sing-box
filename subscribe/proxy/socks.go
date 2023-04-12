package proxy

import (
	C "github.com/sagernet/sing-box/constant"
	option "github.com/sagernet/sing-box/option"
	"net"
	"net/url"
	"strconv"
	"strings"
)

type ProxySocks struct {
	Tag  string // 标签
	Type string // 代理类型
	//
	Dialer option.DialerOptions
	//
	Address      string // IP地址或域名
	Port         uint16 // 端口
	Username     string // 用户名
	Password     string // 密码
	SocksVersion string // SOCKS版本
}

func (p *ProxySocks) GetType() string {
	return C.TypeSocks
}

func (p *ProxySocks) GetTag() string {
	return p.Tag
}

func (p *ProxySocks) ParseLink(link string) error {
	u, err := url.Parse(link)
	if err != nil {
		return err
	}

	p.Address = u.Hostname()
	if u.Port() == "" {
		p.Port = 80
	} else {
		portUint16, err := strconv.ParseUint(u.Port(), 10, 16)
		if err != nil {
			return err
		}
		p.Port = uint16(portUint16)
	}

	p.Username = u.User.Username()
	p.Password, _ = u.User.Password()

	if strings.Index(u.Scheme, "4") > 0 {
		p.SocksVersion = "4"
	} else {
		p.SocksVersion = "5"
	}

	p.Tag = u.Fragment

	if p.Tag == "" {
		p.Tag = net.JoinHostPort(p.Address, strconv.FormatUint(uint64(p.Port), 10))
	}

	p.Type = C.TypeSocks

	return nil
}

func (p *ProxySocks) SetDialer(dialer option.DialerOptions) {
	p.Dialer = dialer
}

func (p *ProxySocks) GenerateOutboundOptions() (option.Outbound, error) {
	out := option.Outbound{
		Tag:  p.Tag,
		Type: C.TypeSocks,
		SocksOptions: option.SocksOutboundOptions{
			ServerOptions: option.ServerOptions{
				Server:     p.Address,
				ServerPort: p.Port,
			},
			Username: p.Username,
			Password: p.Password,
			Version:  p.SocksVersion,
		},
	}

	out.SocksOptions.DialerOptions = p.Dialer

	return out, nil
}
