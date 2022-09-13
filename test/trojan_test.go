package main

import (
	"net/netip"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

func TestTrojanOutbound(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startDockerContainer(t, DockerOptions{
		Image: ImageTrojan,
		Ports: []uint16{serverPort, testPort},
		Bind: map[string]string{
			"trojan.json": "/config/config.json",
			certPem:       "/path/to/certificate.crt",
			keyPem:        "/path/to/private.key",
		},
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
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeTrojan,
				TrojanOptions: option.TrojanOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					Password: "password",
					TLS: &option.OutboundTLSOptions{
						Enabled:         true,
						ServerName:      "example.org",
						CertificatePath: certPem,
					},
				},
			},
		},
	})
	testSuit(t, clientPort, testPort)
}

func TestTrojanSelf(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				MixedOptions: option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.ListenAddress(netip.IPv4Unspecified()),
						ListenPort: clientPort,
					},
				},
			},
			{
				Type: C.TypeTrojan,
				TrojanOptions: option.TrojanInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.ListenAddress(netip.IPv4Unspecified()),
						ListenPort: serverPort,
					},
					Users: []option.TrojanUser{
						{
							Name:     "sekai",
							Password: "password",
						},
					},
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
				Type: C.TypeTrojan,
				Tag:  "trojan-out",
				TrojanOptions: option.TrojanOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
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
					DefaultOptions: option.DefaultRule{
						Inbound:  []string{"mixed-in"},
						Outbound: "trojan-out",
					},
				},
			},
		},
	})
	testSuit(t, clientPort, testPort)
}

func TestTrojanPlainSelf(t *testing.T) {
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				MixedOptions: option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.ListenAddress(netip.IPv4Unspecified()),
						ListenPort: clientPort,
					},
				},
			},
			{
				Type: C.TypeTrojan,
				TrojanOptions: option.TrojanInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.ListenAddress(netip.IPv4Unspecified()),
						ListenPort: serverPort,
					},
					Users: []option.TrojanUser{
						{
							Name:     "sekai",
							Password: "password",
						},
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
			{
				Type: C.TypeTrojan,
				Tag:  "trojan-out",
				TrojanOptions: option.TrojanOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					Password: "password",
				},
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					DefaultOptions: option.DefaultRule{
						Inbound:  []string{"mixed-in"},
						Outbound: "trojan-out",
					},
				},
			},
		},
	})
	testSuit(t, clientPort, testPort)
}
