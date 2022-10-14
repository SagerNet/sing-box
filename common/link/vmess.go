package link

import (
	"strings"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

type vmess struct {
	Ver      string `json:"v,omitempty"`
	Add      string `json:"add,omitempty"`
	Aid      int    `json:"aid,omitempty"`
	Host     string `json:"host,omitempty"`
	ID       string `json:"id,omitempty"`
	Net      string `json:"net,omitempty"`
	Path     string `json:"path,omitempty"`
	Port     uint16 `json:"port,omitempty"`
	Ps       string `json:"ps,omitempty"`
	TLS      string `json:"tls,omitempty"`
	Type     string `json:"type,omitempty"`
	OrigLink string `json:"-,omitempty"`
}

// Options implements Link
func (v *vmess) Options() *option.Outbound {
	out := &option.Outbound{
		Type: "vmess",
		Tag:  v.Ps,
		VMessOptions: option.VMessOutboundOptions{
			ServerOptions: option.ServerOptions{
				Server:     v.Add,
				ServerPort: v.Port,
			},
			UUID:     v.ID,
			AlterId:  v.Aid,
			Security: "auto",
		},
	}

	opt := &option.V2RayTransportOptions{}

	switch v.Net {
	case "":
		opt = nil
	case "tcp", "h2", "http":
		opt.Type = C.V2RayTransportTypeHTTP
		opt.HTTPOptions.Path = strings.Split(v.Path, ",")[0]
		if v.Host != "" {
			opt.HTTPOptions.Host = strings.Split(v.Host, ",")
			opt.HTTPOptions.Headers["Host"] = opt.HTTPOptions.Host[0]
		}
	case "ws":
		opt.Type = C.V2RayTransportTypeWebsocket
		opt.WebsocketOptions.Path = v.Path
		opt.WebsocketOptions.Headers = map[string]string{
			"Host": v.Host,
		}
	}

	if v.TLS == "tls" {
		out.VMessOptions.TLS = &option.OutboundTLSOptions{
			Insecure:   true,
			ServerName: v.Host,
		}
	}

	out.VMessOptions.Transport = opt
	return out
}
