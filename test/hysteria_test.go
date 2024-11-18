package main

import (
	"net/netip"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/json/badoption"
)

func TestHysteriaSelf(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: clientPort,
					},
				},
			},
			{
				Type: C.TypeHysteria,
				Options: &option.HysteriaInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					UpMbps:   100,
					DownMbps: 100,
					Users: []option.HysteriaUser{{
						AuthString: "password",
					}},
					Obfs: "fuck me till the daylight",
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
						TLS: &option.InboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
							KeyPath:         keyPem,
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
				Type: C.TypeHysteria,
				Tag:  "hy-out",
				Options: &option.HysteriaOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					UpMbps:     100,
					DownMbps:   100,
					AuthString: "password",
					Obfs:       "fuck me till the daylight",
					OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
						TLS: &option.OutboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
						},
					},
				},
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RawDefaultRule: option.RawDefaultRule{
							Inbound: []string{"mixed-in"},
						},
						RuleAction: option.RuleAction{
							Action: C.RuleActionTypeRoute,

							RouteOptions: option.RouteActionOptions{
								Outbound: "hy-out",
							},
						},
					},
				},
			},
		},
	})
	testSuit(t, clientPort, testPort)
}

func TestHysteriaInbound(t *testing.T) {
	caPem, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeHysteria,
				Options: &option.HysteriaInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					UpMbps:   100,
					DownMbps: 100,
					Users: []option.HysteriaUser{{
						AuthString: "password",
					}},
					Obfs: "fuck me till the daylight",
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
						TLS: &option.InboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
							KeyPath:         keyPem,
						},
					},
				},
			},
		},
	})
	startDockerContainer(t, DockerOptions{
		Image: ImageHysteria,
		Ports: []uint16{serverPort, clientPort},
		Cmd:   []string{"-c", "/etc/hysteria/config.json", "client"},
		Bind: map[string]string{
			"hysteria-client.json": "/etc/hysteria/config.json",
			caPem:                  "/etc/hysteria/ca.pem",
		},
	})
	testSuit(t, clientPort, testPort)
}

func TestHysteriaOutbound(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startDockerContainer(t, DockerOptions{
		Image: ImageHysteria,
		Ports: []uint16{testPort},
		Cmd:   []string{"-c", "/etc/hysteria/config.json", "server"},
		Bind: map[string]string{
			"hysteria-server.json": "/etc/hysteria/config.json",
			certPem:                "/etc/hysteria/cert.pem",
			keyPem:                 "/etc/hysteria/key.pem",
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
				Type: C.TypeHysteria,
				Options: &option.HysteriaOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					UpMbps:     100,
					DownMbps:   100,
					AuthString: "password",
					Obfs:       "fuck me till the daylight",
					OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
						TLS: &option.OutboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
						},
					},
				},
			},
		},
	})
	testSuitSimple1(t, clientPort, testPort)
}
