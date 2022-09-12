package sip003

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/v2ray"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func init() {
	RegisterPlugin("v2ray-plugin", newV2RayPlugin)
}

func newV2RayPlugin(pluginOpts Args, router adapter.Router, dialer N.Dialer, serverAddr M.Socksaddr) (Plugin, error) {
	var tlsOptions option.OutboundTLSOptions
	if _, loaded := pluginOpts.Get("tls"); loaded {
		tlsOptions.Enabled = true
	}
	if certPath, certLoaded := pluginOpts.Get("cert"); certLoaded {
		tlsOptions.CertificatePath = certPath
	}
	if certRaw, certLoaded := pluginOpts.Get("certRaw"); certLoaded {
		certHead := "-----BEGIN CERTIFICATE-----"
		certTail := "-----END CERTIFICATE-----"
		fixedCert := certHead + "\n" + certRaw + "\n" + certTail
		tlsOptions.Certificate = fixedCert
	}

	mode := "websocket"
	if modeOpt, loaded := pluginOpts.Get("mode"); loaded {
		mode = modeOpt
	}

	host := "cloudfront.com"
	path := "/"

	if hostOpt, loaded := pluginOpts.Get("host"); loaded {
		host = hostOpt
	}
	if pathOpt, loaded := pluginOpts.Get("path"); loaded {
		path = pathOpt
	}

	var tlsClient tls.Config
	var err error
	if tlsOptions.Enabled {
		tlsClient, err = tls.NewClient(router, serverAddr.AddrString(), tlsOptions)
		if err != nil {
			return nil, err
		}
	}

	var transportOptions option.V2RayTransportOptions
	switch mode {
	case "websocket":
		transportOptions = option.V2RayTransportOptions{
			Type: C.V2RayTransportTypeWebsocket,
			WebsocketOptions: option.V2RayWebsocketOptions{
				Headers: map[string]string{
					"Host": host,
				},
				Path: path,
			},
		}
	case "quic":
		transportOptions = option.V2RayTransportOptions{
			Type: C.V2RayTransportTypeQUIC,
		}
	default:
		return nil, E.New("v2ray-plugin: unknown mode: " + mode)
	}

	return v2ray.NewClientTransport(context.Background(), dialer, serverAddr, transportOptions, tlsClient)
}
