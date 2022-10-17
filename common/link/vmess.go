package link

import (
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

// Vmess is the base struct of vmess link
type Vmess struct {
	Tag              string
	Server           string
	ServerPort       uint16
	UUID             string
	AlterID          int
	Security         string
	Transport        string
	TransportPath    string
	Host             string
	TLS              bool
	TLSAllowInsecure bool
}

// Options implements Link
func (v *Vmess) Options() *option.Outbound {
	out := &option.Outbound{
		Type: C.TypeVMess,
		Tag:  v.Tag,
		VMessOptions: option.VMessOutboundOptions{
			ServerOptions: option.ServerOptions{
				Server:     v.Server,
				ServerPort: v.ServerPort,
			},
			UUID:     v.UUID,
			AlterId:  v.AlterID,
			Security: v.Security,
		},
	}

	if v.TLS {
		out.VMessOptions.TLS = &option.OutboundTLSOptions{
			Enabled:    true,
			Insecure:   v.TLSAllowInsecure,
			ServerName: v.Host,
		}
	}

	opt := &option.V2RayTransportOptions{
		Type: v.Transport,
	}

	switch v.Transport {
	case "":
		opt = nil
	case C.V2RayTransportTypeHTTP:
		opt.HTTPOptions.Path = v.TransportPath
		if v.Host != "" {
			opt.HTTPOptions.Host = []string{v.Host}
			opt.HTTPOptions.Headers["Host"] = v.Host
		}
	case C.V2RayTransportTypeWebsocket:
		opt.WebsocketOptions.Path = v.TransportPath
		opt.WebsocketOptions.Headers = map[string]string{
			"Host": v.Host,
		}
	case C.V2RayTransportTypeQUIC:
		// do nothing
	case C.V2RayTransportTypeGRPC:
		opt.GRPCOptions.ServiceName = v.Host
	}

	out.VMessOptions.Transport = opt
	return out
}
