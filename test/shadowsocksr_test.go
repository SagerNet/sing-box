package main

import (
	"net/netip"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

func TestShadowsocksR(t *testing.T) {
	startDockerContainer(t, DockerOptions{
		Image: ImageShadowsocksR,
		Ports: []uint16{serverPort, testPort},
		Bind: map[string]string{
			"shadowsocksr.json": "/etc/shadowsocks-r/config.json",
		},
	})
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				MixedOptions: option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.NewListenAddress(netip.IPv4Unspecified()),
						ListenPort: clientPort,
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeShadowsocksR,
				ShadowsocksROptions: option.ShadowsocksROutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					Method:   "aes-256-cfb",
					Password: "password0",
					Obfs:     "plain",
					Protocol: "origin",
				},
			},
		},
	})
	testSuit(t, clientPort, testPort)
}
