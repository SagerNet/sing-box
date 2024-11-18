package main

import (
	"net/netip"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-shadowsocks2/shadowstream"
	"github.com/sagernet/sing/common"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/json/badoption"
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
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: clientPort,
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeShadowsocks,
				Options: &option.ShadowsocksOutboundOptions{
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
