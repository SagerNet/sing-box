package proxy

import (
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"net"
	"net/url"
	"strconv"
	"strings"
)

type ProxyTrojan struct {
	Tag  string // 标签
	Type string // 代理类型
	//
	Dialer option.DialerOptions
	//
	Address  string // IP地址或域名
	Port     uint16 // 端口
	Password string // 密码
	SNI      string // SNI
}

func (p *ProxyTrojan) GetTag() string {
	return p.Tag
}

func (p *ProxyTrojan) GetType() string {
	return C.TypeVMess
}

func (p *ProxyTrojan) ParseLink(link string) error {
	link = strings.TrimPrefix(link, "trojan://")
	u, err := url.Parse("http://" + link)
	if err != nil {
		return E.New("invalid trojan link: ", err.Error())
	}
	if u.User == nil || u.User.Username() == "" {
		return E.New("invalid trojan link")
	}
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		host = u.Hostname()
		port = "443"
	}
	portUint, err := strconv.ParseUint(port, 10, 16)
	if err != nil || portUint == 0 || portUint > 65535 {
		return E.New("invalid trojan link: ", err.Error())
	}

	p.Address = host
	p.Port = uint16(portUint)
	p.Password = u.User.Username()
	p.SNI = u.Query().Get("sni")

	if u.Fragment != "" {
		p.Tag = u.Fragment
	} else {
		p.Tag = u.Host
	}

	p.Type = C.TypeTrojan

	return nil
}

func (p *ProxyTrojan) SetDialer(dialer option.DialerOptions) {
	p.Dialer = dialer
}

func (p *ProxyTrojan) GenerateOutboundOptions() (option.Outbound, error) {
	out := option.Outbound{
		Tag:  p.Tag,
		Type: C.TypeTrojan,
		TrojanOptions: option.TrojanOutboundOptions{
			ServerOptions: option.ServerOptions{
				Server:     p.Address,
				ServerPort: p.Port,
			},
			Password: p.Password,
			TLS: &option.OutboundTLSOptions{
				Enabled:    true,
				ServerName: p.SNI,
			},
		},
	}

	out.TrojanOptions.DialerOptions = p.Dialer

	return out, nil
}
