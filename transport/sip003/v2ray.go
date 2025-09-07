package sip003

import (
	"context"
	"net"
	"strconv"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/v2ray"
	"github.com/sagernet/sing-vmess"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json/badoption"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func init() {
	RegisterPlugin("v2ray-plugin", newV2RayPlugin)
}

func newV2RayPlugin(ctx context.Context, pluginOpts Args, router adapter.Router, dialer N.Dialer, serverAddr M.Socksaddr) (Plugin, error) {
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
		tlsOptions.Certificate = []string{fixedCert}
	}

	mode := "websocket"
	if modeOpt, loaded := pluginOpts.Get("mode"); loaded {
		mode = modeOpt
	}

	host := "cloudfront.com"
	path := "/"

	if hostOpt, loaded := pluginOpts.Get("host"); loaded {
		host = hostOpt
		tlsOptions.ServerName = hostOpt
	}
	if pathOpt, loaded := pluginOpts.Get("path"); loaded {
		path = pathOpt
	}

	var tlsClient tls.Config
	var err error
	if tlsOptions.Enabled {
		tlsClient, err = tls.NewClient(ctx, logger.NOP(), serverAddr.AddrString(), tlsOptions)
		if err != nil {
			return nil, err
		}
	}

	var mux int
	var transportOptions option.V2RayTransportOptions
	switch mode {
	case "websocket":
		transportOptions = option.V2RayTransportOptions{
			Type: C.V2RayTransportTypeWebsocket,
			WebsocketOptions: option.V2RayWebsocketOptions{
				Headers: map[string]badoption.Listable[string]{
					"Host": []string{host},
				},
				Path: path,
			},
		}
		if muxOpt, loaded := pluginOpts.Get("mux"); loaded {
			muxVal, err := strconv.Atoi(muxOpt)
			if err != nil {
				return nil, E.Cause(err, "parse mux value")
			}
			mux = muxVal
		} else {
			mux = 1
		}
	case "quic":
		transportOptions = option.V2RayTransportOptions{
			Type: C.V2RayTransportTypeQUIC,
		}
	default:
		return nil, E.New("v2ray-plugin: unknown mode: " + mode)
	}

	transport, err := v2ray.NewClientTransport(context.Background(), dialer, serverAddr, transportOptions, tlsClient)
	if err != nil {
		return nil, err
	}

	if mux > 0 {
		return &v2rayMuxWrapper{transport}, nil
	}

	return transport, nil
}

var _ Plugin = (*v2rayMuxWrapper)(nil)

type v2rayMuxWrapper struct {
	adapter.V2RayClientTransport
}

func (w *v2rayMuxWrapper) DialContext(ctx context.Context) (net.Conn, error) {
	conn, err := w.V2RayClientTransport.DialContext(ctx)
	if err != nil {
		return nil, err
	}
	return vmess.NewMuxConnWrapper(conn, vmess.MuxDestination), nil
}
