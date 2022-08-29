package main

import (
	"net/netip"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	F "github.com/sagernet/sing/common/format"
)

func TestShadowTLSOutbound(t *testing.T) {
	startDockerContainer(t, DockerOptions{
		Image:      ImageShadowTLS,
		Ports:      []uint16{serverPort, otherPort},
		EntryPoint: "shadow-tls",
		Cmd:        []string{"--threads", "1", "server", "0.0.0.0:" + F.ToString(serverPort), "127.0.0.1:" + F.ToString(otherPort), "google.com:443"},
	})
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				MixedOptions: option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.ListenAddress(netip.IPv4Unspecified()),
						ListenPort: clientPort,
					},
				},
			},
			{
				Type: C.TypeMixed,
				Tag:  "detour",
				MixedOptions: option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.ListenAddress(netip.IPv4Unspecified()),
						ListenPort: otherPort,
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeSocks,
				SocksOptions: option.SocksOutboundOptions{
					OutboundDialerOptions: option.OutboundDialerOptions{
						DialerOptions: option.DialerOptions{
							Detour: "detour",
						},
					},
				},
			},
			{
				Type: C.TypeShadowTLS,
				Tag:  "detour",
				ShadowTLSOptions: option.ShadowTLSOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					TLS: &option.OutboundTLSOptions{
						Enabled:    true,
						ServerName: "google.com",
						MaxVersion: "1.2",
					},
				},
			},
			{
				Type: C.TypeDirect,
				Tag:  "direct",
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{{
				DefaultOptions: option.DefaultRule{
					Inbound:  []string{"detour"},
					Outbound: "direct",
				},
			}},
		},
	})
	testTCP(t, clientPort, testPort)
}
