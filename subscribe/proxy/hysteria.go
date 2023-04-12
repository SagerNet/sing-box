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

type ProxyHysteria struct {
	Tag  string // 标签
	Type string // 代理类型
	//
	Dialer option.DialerOptions
	//
	Address   string   // IP地址或域名
	Port      uint16   // 端口
	Protocol  string   // 协议 udp/wechat-video/faketcp
	AuthStr   string   // 认证字符串
	SNI       string   // TLS SNI
	Insecure  bool     // TLS Insecure
	UpMbps    uint64   // 上行速度
	DownMbps  uint64   // 下行速度
	ALPN      []string // QUIC TLS ALPN
	Obfs      string   // 混淆
	ObfsParam string   // 混淆参数
}

func (p *ProxyHysteria) GetType() string {
	return C.TypeHysteria
}

func (p *ProxyHysteria) GetTag() string {
	return p.Tag
}

func (p *ProxyHysteria) ParseLink(link string) error {
	configStr := strings.TrimPrefix(link, "hysteria://")
	u, err := url.Parse("http://" + configStr)
	if err != nil {
		return E.New("invalid hysteria link")
	}
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		return E.New("invalid hysteria link")
	}
	portUint, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return E.New("invalid hysteria link")
	}
	query := u.Query()
	if query == nil {
		return E.New("invalid hysteria link")
	}

	p.Address = host
	p.Port = uint16(portUint)

	if query.Get("protocol") != "" {
		p.Protocol = query.Get("protocol")
	} else {
		p.Protocol = "udp"
	}

	if query.Get("auth") != "" {
		p.AuthStr = query.Get("auth")
	}
	if query.Get("upmbps") != "" {
		upMbpsStr := query.Get("upmbps")
		upMbps, err := strconv.ParseUint(upMbpsStr, 10, 32)
		if err != nil {
			return E.New("invalid hysteria link")
		}
		p.UpMbps = upMbps
	} else {
		return E.New("invalid hysteria link")
	}
	if query.Get("downmbps") != "" {
		downMbpsStr := query.Get("upmbps")
		downMbps, err := strconv.ParseUint(downMbpsStr, 10, 32)
		if err != nil {
			return E.New("invalid hysteria link")
		}
		p.DownMbps = downMbps
	} else {
		return E.New("invalid hysteria link")
	}

	if query.Get("obfs") != "" {
		p.Obfs = query.Get("obfs")
		if query.Get("obfsParam") != "" {
			p.ObfsParam = query.Get("obfsParam")
		}
	}

	if query.Get("peer") != "" {
		p.SNI = query.Get("peer")
	}

	if query.Get("insecure") == "1" {
		p.Insecure = true
	}

	if query.Get("alpn") != "" {
		p.ALPN = []string{query.Get("alpn")}
	}

	if u.Fragment != "" {
		p.Tag = u.Fragment
	} else {
		p.Tag = net.JoinHostPort(host, port)
	}

	p.Type = C.TypeHysteria

	return nil
}

func (p *ProxyHysteria) SetDialer(dialer option.DialerOptions) {
	p.Dialer = dialer
}

func (p *ProxyHysteria) GenerateOutboundOptions() (option.Outbound, error) {

	if p.Protocol != "udp" {
		return option.Outbound{}, E.New("hysteria protocol '", p.Protocol, "' not supported in sing-box")
	}

	/**
	if p.Obfs == "xplus" {
		return nil, E.New("hysteria obfs `%s` not supported in sing-box", p.Obfs)
	}
	*/

	out := option.Outbound{
		Tag:  p.Tag,
		Type: C.TypeHysteria,
		HysteriaOptions: option.HysteriaOutboundOptions{
			ServerOptions: option.ServerOptions{
				Server:     p.Address,
				ServerPort: p.Port,
			},
			AuthString: p.AuthStr,
			UpMbps:     int(p.UpMbps),
			DownMbps:   int(p.DownMbps),
			Obfs:       p.ObfsParam,
			TLS: &option.OutboundTLSOptions{
				Enabled:    true,
				ServerName: p.SNI,
				Insecure:   p.Insecure,
				ALPN:       p.ALPN,
			},
		},
	}

	out.HysteriaOptions.DialerOptions = p.Dialer

	return out, nil
}
