package main

import (
	"net/netip"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-shadowsocks/shadowaead_2022"
	F "github.com/sagernet/sing/common/format"
)

func TestShadowTLS(t *testing.T) {
	method := shadowaead_2022.List[0]
	password := mkBase64(t, 16)
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
				Type: C.TypeShadowTLS,
				Tag:  "in",
				ShadowTLSOptions: option.ShadowTLSInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.ListenAddress(netip.IPv4Unspecified()),
						ListenPort: serverPort,
						Detour:     "detour",
					},
					Handshake: option.ShadowTLSHandshakeOptions{
						ServerOptions: option.ServerOptions{
							Server:     "google.com",
							ServerPort: 443,
						},
					},
				},
			},
			{
				Type: C.TypeShadowsocks,
				Tag:  "detour",
				ShadowsocksOptions: option.ShadowsocksInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.ListenAddress(netip.IPv4Unspecified()),
						ListenPort: otherPort,
					},
					Method:   method,
					Password: password,
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeShadowsocks,
				ShadowsocksOptions: option.ShadowsocksOutboundOptions{
					Method:   method,
					Password: password,
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
