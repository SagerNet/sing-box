package main

import (
	"net/netip"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

func TestShadowsocksObfs(t *testing.T) {
	for _, mode := range []string{
		"http", "tls",
	} {
		t.Run("obfs-local "+mode, func(t *testing.T) {
			testShadowsocksPlugin(t, "obfs-local", "obfs="+mode, "--plugin obfs-server --plugin-opts obfs="+mode)
		})
	}
}

// Since I can't test this on m1 mac (rosetta error: bss_size overflow), I don't care about it
func _TestShadowsocksV2RayPlugin(t *testing.T) {
	testShadowsocksPlugin(t, "v2ray-plugin", "", "--plugin v2ray-plugin --plugin-opts=server")
}

func testShadowsocksPlugin(t *testing.T, name string, opts string, args string) {
	startDockerContainer(t, DockerOptions{
		Image: ImageShadowsocksLegacy,
		Ports: []uint16{serverPort, testPort},
		Env: []string{
			"SS_MODULE=ss-server",
			"SS_CONFIG=-s 0.0.0.0 -u -p 10000 -m chacha20-ietf-poly1305 -k FzcLbKs2dY9mhL " + args,
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
					Method:        "chacha20-ietf-poly1305",
					Password:      "FzcLbKs2dY9mhL",
					Plugin:        name,
					PluginOptions: opts,
				},
			},
		},
	})
	testSuitSimple(t, clientPort, testPort)
}
