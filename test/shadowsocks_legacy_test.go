package main

import (
	"net/netip"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-shadowsocks2/shadowstream"
	F "github.com/sagernet/sing/common/format"
)

func TestShadowsocksLegacy(t *testing.T) {
	testShadowsocksLegacy(t, shadowstream.MethodList[0])
}

func testShadowsocksLegacy(t *testing.T, method string) {
	startDockerContainer(t, DockerOptions{
		Image: ImageShadowsocksLegacy,
		Ports: []uint16{serverPort},
		Env: []string{
			"SS_MODULE=ss-server",
			F.ToString("SS_CONFIG=-s 0.0.0.0 -u -p 10000 -m ", method, " -k FzcLbKs2dY9mhL"),
		},
	})
	startInstance(t, option.Options{
		Inbounds: []option.LegacyInbound{
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
		LegacyOutbounds: []option.LegacyOutbound{
			{
				Type: C.TypeShadowsocks,
				ShadowsocksOptions: option.ShadowsocksOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					Method:   method,
					Password: "FzcLbKs2dY9mhL",
				},
			},
		},
	})
	testSuitSimple(t, clientPort, testPort)
}
