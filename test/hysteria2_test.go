package main

import (
	"net/netip"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/hysteria2"
)

func TestHysteria2Self(t *testing.T) {
	t.Run("self", func(t *testing.T) {
		testHysteria2Self(t, "")
	})
	t.Run("self-salamander", func(t *testing.T) {
		testHysteria2Self(t, "password")
	})
}

func testHysteria2Self(t *testing.T, salamanderPassword string) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	var obfs *option.Hysteria2Obfs
	if salamanderPassword != "" {
		obfs = &option.Hysteria2Obfs{
			Type:     hysteria2.ObfsTypeSalamander,
			Password: salamanderPassword,
		}
	}
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				MixedOptions: option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.NewListenAddress(netip.IPv4Unspecified()),
						ListenPort: clientPort,
					},
				},
			},
			{
				Type: C.TypeHysteria2,
				Hysteria2Options: option.Hysteria2InboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.NewListenAddress(netip.IPv4Unspecified()),
						ListenPort: serverPort,
					},
					Obfs: obfs,
					Users: []option.Hysteria2User{{
						Password: "password",
					}},
					TLS: &option.InboundTLSOptions{
						Enabled:         true,
						ServerName:      "example.org",
						CertificatePath: certPem,
						KeyPath:         keyPem,
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
			{
				Type: C.TypeHysteria2,
				Tag:  "hy2-out",
				Hysteria2Options: option.Hysteria2OutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					Obfs:     obfs,
					Password: "password",
					TLS: &option.OutboundTLSOptions{
						Enabled:         true,
						ServerName:      "example.org",
						CertificatePath: certPem,
					},
				},
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						Inbound:  []string{"mixed-in"},
						Outbound: "hy2-out",
					},
				},
			},
		},
	})
	testSuit(t, clientPort, testPort)
}
