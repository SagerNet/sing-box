package main

import (
	"net/netip"
	"testing"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-quic/hysteria2"
	"github.com/sagernet/sing/common"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/json/badoption"
)

func TestHysteria2Self(t *testing.T) {
	t.Run("self", func(t *testing.T) {
		testHysteria2Self(t, "", false)
	})
	t.Run("self-salamander", func(t *testing.T) {
		testHysteria2Self(t, "password", false)
	})
	t.Run("self-hop", func(t *testing.T) {
		testHysteria2Self(t, "", true)
	})
	t.Run("self-hop-salamander", func(t *testing.T) {
		testHysteria2Self(t, "password", true)
	})
}

func TestHysteria2Hop(t *testing.T) {
	testHysteria2Self(t, "password", true)
}

func testHysteria2Self(t *testing.T, salamanderPassword string, portHop bool) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	var obfs *option.Hysteria2Obfs
	if salamanderPassword != "" {
		obfs = &option.Hysteria2Obfs{
			Type:     hysteria2.ObfsTypeSalamander,
			Password: salamanderPassword,
		}
	}
	var (
		serverPorts []string
		hopInterval time.Duration
	)
	if portHop {
		serverPorts = []string{F.ToString(serverPort, ":", serverPort)}
		hopInterval = 5 * time.Second
	}
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
				Type: C.TypeHysteria2,
				Options: &option.Hysteria2InboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					UpMbps:   100,
					DownMbps: 100,
					Obfs:     obfs,
					Users: []option.Hysteria2User{{
						Password: "password",
					}},
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
				Type: C.TypeHysteria2,
				Tag:  "hy2-out",
				Options: &option.Hysteria2OutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					ServerPorts: serverPorts,
					HopInterval: badoption.Duration(hopInterval),
					UpMbps:      100,
					DownMbps:    100,
					Obfs:        obfs,
					Password:    "password",
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
								Outbound: "hy2-out",
							},
						},
					},
				},
			},
		},
	})
	testSuitLargeUDP(t, clientPort, testPort)
	if portHop {
		time.Sleep(5 * time.Second)
		testSuitLargeUDP(t, clientPort, testPort)
	}
}

func TestHysteria2Inbound(t *testing.T) {
	caPem, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeHysteria2,
				Options: &option.Hysteria2InboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					Obfs: &option.Hysteria2Obfs{
						Type:     hysteria2.ObfsTypeSalamander,
						Password: "cry_me_a_r1ver",
					},
					Users: []option.Hysteria2User{{
						Password: "password",
					}},
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
		Image: ImageHysteria2,
		Ports: []uint16{serverPort, clientPort},
		Cmd:   []string{"client", "-c", "/etc/hysteria/config.yml", "--disable-update-check", "--log-level", "debug"},
		Bind: map[string]string{
			"hysteria2-client.yml": "/etc/hysteria/config.yml",
			caPem:                  "/etc/hysteria/ca.pem",
		},
	})
	testSuit(t, clientPort, testPort)
}

func TestHysteria2Outbound(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startDockerContainer(t, DockerOptions{
		Image: ImageHysteria2,
		Ports: []uint16{testPort},
		Cmd:   []string{"server", "-c", "/etc/hysteria/config.yml", "--disable-update-check", "--log-level", "debug"},
		Bind: map[string]string{
			"hysteria2-server.yml": "/etc/hysteria/config.yml",
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
				Type: C.TypeHysteria2,
				Options: &option.Hysteria2OutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					Obfs: &option.Hysteria2Obfs{
						Type:     hysteria2.ObfsTypeSalamander,
						Password: "cry_me_a_r1ver",
					},
					Password: "password",
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
