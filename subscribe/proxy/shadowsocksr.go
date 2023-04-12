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

type ProxyShadowsocksR struct {
	Tag  string // 标签
	Type string // 代理类型
	//
	Dialer option.DialerOptions
	//
	Address       string // IP地址或域名
	Port          uint16 // 端口
	Method        string // 加密方式
	Password      string // 密码
	Obfs          string // 混淆
	ObfsParam     string // 混淆参数
	Protocol      string // 协议
	ProtocolParam string // 协议参数
}

func (p *ProxyShadowsocksR) GetType() string {
	return C.TypeShadowsocksR
}

func (p *ProxyShadowsocksR) GetTag() string {
	return p.Tag
}

func (p *ProxyShadowsocksR) ParseLink(link string) error {
	link = strings.TrimPrefix(link, "ssr://")
	uriDecodeBytes, err := base64.StdEncoding.DecodeString(link)
	if err != nil {
		return E.New("invalid shadowsocksR link: ", err.Error())
	}
	uriSlice := strings.Split(string(uriDecodeBytes), "/")
	userInfo := strings.Split(uriSlice[0], ":")
	if len(userInfo) != 6 {
		return E.New("invalid shadowsocksR link")
	}
	_, params, found := strings.Cut(uriSlice[1], "?")
	if !found {
		return E.New("invalid shadowsocksR link")
	}
	query, err := url.ParseQuery(params)
	if err != nil {
		return E.New("invalid shadowsocksR link: ", err.Error())
	}
	var (
		protoParam string
		obfsParam  string
		remarks    string
	)
	if query.Get("protoparam") != "" {
		protoParamBytes, err := base64.StdEncoding.DecodeString(query.Get("protoparam"))
		if err == nil {
			protoParam = string(protoParamBytes)
		}
	}
	if query.Get("obfsparam") != "" {
		obfsParamBytes, err := base64.StdEncoding.DecodeString(query.Get("obfsparam"))
		if err == nil {
			obfsParam = string(obfsParamBytes)
		}
	}
	if query.Get("remarks") != "" {
		remarksBytes, err := base64.StdEncoding.DecodeString(query.Get("remarks"))
		if err == nil {
			remarks = string(remarksBytes)
		}
	}
	var (
		server         = userInfo[0]
		serverPort     = userInfo[1]
		protocol       = userInfo[2]
		method         = userInfo[3]
		obfs           = userInfo[4]
		passwordBase64 = userInfo[5]
		password       string
	)
	passwordBytes, err := base64.StdEncoding.DecodeString(passwordBase64)
	if err != nil {
		return E.New("invalid shadowsocksR link")
	}
	password = string(passwordBytes)
	portUint, err := strconv.ParseUint(serverPort, 10, 16)
	if err != nil || portUint == 0 || portUint > 65535 {
		return E.New("invalid shadowsocksR link")
	}
	if remarks == "" {
		remarks = net.JoinHostPort(server, serverPort)
	}

	p.Address = server
	p.Port = uint16(portUint)
	p.Method = method
	p.Password = password
	p.Obfs = obfs
	p.ObfsParam = obfsParam
	p.Protocol = protocol
	p.ProtocolParam = protoParam

	p.Tag = remarks

	p.Type = C.TypeShadowsocksR

	return nil
}

func (p *ProxyShadowsocksR) SetDialer(dialer option.DialerOptions) {
	p.Dialer = dialer
}

func (p *ProxyShadowsocksR) GenerateOutboundOptions() (option.Outbound, error) {
	out := option.Outbound{
		Tag:  p.Tag,
		Type: C.TypeShadowsocksR,
		ShadowsocksROptions: option.ShadowsocksROutboundOptions{
			ServerOptions: option.ServerOptions{
				Server:     p.Address,
				ServerPort: p.Port,
			},
			Method:        p.Method,
			Password:      p.Password,
			Obfs:          p.Obfs,
			ObfsParam:     p.ObfsParam,
			Protocol:      p.Protocol,
			ProtocolParam: p.ProtocolParam,
		},
	}

	out.ShadowsocksROptions.DialerOptions = p.Dialer

	return out, nil
}
