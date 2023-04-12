package proxy

import (
	"encoding/json"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"net"
	"strconv"
	"strings"
)

type ProxyVMess struct {
	Tag  string // 标签
	Type string // 代理类型
	//
	Dialer option.DialerOptions
	//
	Address   string   // IP地址或域名
	Port      uint16   // 端口
	UUID      string   // UUID
	AlterID   uint16   // AlterID
	Encrypt   string   // 加密方式
	TLSEnable bool     // 是否启用TLS
	TLSSNI    string   // TLS SNI
	TLSALPN   []string // TLS ALPN
	//
	TransportMode string // 传输方式  TCP/KCP/WS/H2/QUIC/GRPC
	//
	TransportFakeType string // 伪装类型 none/http/srtp/utp/wechat-video
	//
	TransportTCPHost []string // TCP Host
	//
	TransportH2Host string // HTTP/2 Host
	TransportH2Path string // HTTP/2 Path
	//
	TransportWSHost            string // WebSocket Host
	TransportWSEarlyDataHeader string // WebSocket Early-Data Header
	TransportWSPath            string // WebSocket Path
	//
	TransportGRPCServiceName string // gRPC Service Name
	//
	TransportQUICSecurity string // QUIC Security
	TransportQUICKey      string // QUIC Key
	//
	TransportKCPSeed string // KCP Seed
}

type configVMessJSON struct {
	Version  string `json:"v"`
	Tag      string `json:"ps"`
	Address  string `json:"add"`
	Port     string `json:"port"`
	UUID     string `json:"id"`
	AlterID  string `json:"aid"`
	Security string `json:"scy"`
	Network  string `json:"net"`
	Type     string `json:"type"`
	Host     string `json:"host"`
	Path     string `json:"path"`
	TLS      string `json:"tls"`
	SNI      string `json:"sni"`
}

func (p *ProxyVMess) GetTag() string {
	return p.Tag
}

func (p *ProxyVMess) GetType() string {
	return C.TypeVMess
}

func (p *ProxyVMess) ParseLink(link string) error {
	configStr := strings.TrimPrefix(link, "vmess://")

	configDec, err := Base64Decode(configStr)
	if err != nil {
		return err
	}

	var j configVMessJSON

	err = json.Unmarshal([]byte(configDec), &j)
	if err != nil {
		return err
	}

	p.Address = j.Address
	portUint16, err := strconv.ParseUint(j.Port, 10, 16)
	if err != nil {
		return err
	}
	p.Port = uint16(portUint16)

	p.UUID = j.UUID

	if j.AlterID != "" {
		alterIDInt, err := strconv.ParseInt(j.AlterID, 10, 16)
		if err != nil {
			return err
		}
		p.AlterID = uint16(alterIDInt)
	}

	if j.Security != "" {
		p.Encrypt = j.Security
	} else {
		p.Encrypt = "auto"
	}

	switch j.Network {
	case "tcp":
		p.TransportMode = "TCP"
		if j.Type != "" {
			p.TransportFakeType = j.Type
		}
		if j.Host != "" {
			p.TransportTCPHost = strings.Split(j.Host, ",")
		}
	case "kcp":
		p.TransportMode = "KCP"
		if j.Type != "" {
			p.TransportFakeType = j.Type
		}
		if j.Path != "" {
			p.TransportKCPSeed = j.Path
		}
	case "ws":
		p.TransportMode = "WS"
		if j.Host != "" {
			p.TransportWSHost = j.Host
		}
		if j.Path != "" {
			p.TransportWSPath = j.Path
		}
	case "h2":
		p.TransportMode = "H2"
		if j.Host != "" {
			p.TransportH2Host = j.Host
		}
		if j.Path != "" {
			p.TransportH2Path = j.Path
		}
	case "quic":
		p.TransportMode = "QUIC"
		if j.Type != "" {
			p.TransportFakeType = j.Type
		}
		if j.Host != "" {
			p.TransportQUICSecurity = j.Host
		}
		if j.Path != "" {
			p.TransportQUICKey = j.Path
		}
	case "grpc":
		p.TransportMode = "GRPC"
		if j.Path != "" {
			p.TransportGRPCServiceName = j.Path
		}
	default:
		return E.New("unknown net: ", j.Network)
	}

	if j.TLS != "" {
		p.TLSEnable = true
		if j.SNI != "" {
			p.TLSSNI = j.SNI
		}
	}

	if j.Tag != "" {
		p.Tag = j.Tag
	} else {
		p.Tag = net.JoinHostPort(p.Address, strconv.Itoa(int(p.Port)))
	}

	p.Type = C.TypeVMess

	return nil
}

func (p *ProxyVMess) SetDialer(dialer option.DialerOptions) {
	p.Dialer = dialer
}

func (p *ProxyVMess) GenerateOutboundOptions() (option.Outbound, error) {

	if p.TransportMode == "KCP" {
		return option.Outbound{}, E.New("vmess kcp not supported in sing-box")
	}

	if p.TransportFakeType != "" && p.TransportFakeType != "none" && p.TransportFakeType != "http" {
		return option.Outbound{}, E.New("vmess fake type `%s` not supported in sing-box", p.TransportFakeType)
	}

	out := option.Outbound{
		Tag:  p.Tag,
		Type: C.TypeVMess,
		VMessOptions: option.VMessOutboundOptions{
			ServerOptions: option.ServerOptions{
				Server:     p.Address,
				ServerPort: p.Port,
			},
			UUID:     p.UUID,
			Security: p.Encrypt,
			AlterId:  int(p.AlterID),
		},
	}

	switch p.TransportMode {
	case "TCP":
		if p.TransportTCPHost != nil {
			out.VMessOptions.Transport = &option.V2RayTransportOptions{
				Type: C.V2RayTransportTypeHTTP,
				HTTPOptions: option.V2RayHTTPOptions{
					Host: p.TransportTCPHost,
				},
			}
		}
	case "WS":
		out.VMessOptions.Transport = &option.V2RayTransportOptions{
			Type: C.V2RayTransportTypeWebsocket,
			WebsocketOptions: option.V2RayWebsocketOptions{
				Path: p.TransportWSPath,
				Headers: map[string]option.Listable[string]{
					"Host": {p.TransportWSHost},
				},
			},
		}
	case "H2":
		out.VMessOptions.Transport = &option.V2RayTransportOptions{
			Type: C.V2RayTransportTypeHTTP,
			HTTPOptions: option.V2RayHTTPOptions{
				Host: p.TransportTCPHost,
				Path: p.TransportH2Path,
			},
		}
		if !p.TLSEnable {
			return option.Outbound{}, E.New("vmess h2 must enable tls")
		}
	case "QUIC":
		out.VMessOptions.Transport = &option.V2RayTransportOptions{
			Type:        C.V2RayTransportTypeQUIC,
			QUICOptions: option.V2RayQUICOptions{},
		}
		if !p.TLSEnable {
			return option.Outbound{}, E.New("vmess quic must enable tls")
		}
	case "GRPC":
		out.VMessOptions.Transport = &option.V2RayTransportOptions{
			Type: C.V2RayTransportTypeGRPC,
			GRPCOptions: option.V2RayGRPCOptions{
				ServiceName: p.TransportGRPCServiceName,
			},
		}
	}

	if p.TLSEnable {
		out.VMessOptions.TLS = &option.OutboundTLSOptions{
			Enabled:    true,
			ServerName: p.TLSSNI,
			ALPN:       p.TLSALPN,
		}
	}

	out.VMessOptions.DialerOptions = p.Dialer

	return out, nil
}
