package main

import (
	"net/netip"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

func TestHysteriaSelf(t *testing.T) {
	if !C.QUIC_AVAILABLE {
		t.Skip("QUIC not included")
	}
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startInstance(t, option.Options{
		Log: &option.LogOptions{
			Level: "trace",
		},
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
				Type: C.TypeHysteria,
				HysteriaOptions: option.HysteriaInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.ListenAddress(netip.IPv4Unspecified()),
						ListenPort: serverPort,
					},
					UpMbps:     100,
					DownMbps:   100,
					AuthString: "password",
					Obfs:       "fuck me till the daylight",
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
				Type: C.TypeHysteria,
				Tag:  "hy-out",
				HysteriaOptions: option.HysteriaOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					UpMbps:     100,
					DownMbps:   100,
					AuthString: "password",
					Obfs:       "fuck me till the daylight",
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
						Outbound: "hy-out",
					},
				},
			},
		},
	})
	testSuitHy(t, clientPort, testPort)
}

func TestHysteriaInbound(t *testing.T) {
	if !C.QUIC_AVAILABLE {
		t.Skip("QUIC not included")
	}
	caPem, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startInstance(t, option.Options{
		Log: &option.LogOptions{
			Level: "trace",
		},
		Inbounds: []option.Inbound{
			{
				Type: C.TypeHysteria,
				HysteriaOptions: option.HysteriaInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.ListenAddress(netip.IPv4Unspecified()),
						ListenPort: serverPort,
					},
					UpMbps:     100,
					DownMbps:   100,
					AuthString: "password",
					Obfs:       "fuck me till the daylight",
					TLS: &option.InboundTLSOptions{
						Enabled:         true,
						ServerName:      "example.org",
						CertificatePath: certPem,
						KeyPath:         keyPem,
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
	if !C.QUIC_AVAILABLE {
		t.Skip("QUIC not included")
	}
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startDockerContainer(t, DockerOptions{
		Image: ImageHysteria,
		Ports: []uint16{serverPort, testPort},
		Cmd:   []string{"-c", "/etc/hysteria/config.json", "server"},
		Bind: map[string]string{
			"hysteria-server.json": "/etc/hysteria/config.json",
			certPem:                "/etc/hysteria/cert.pem",
			keyPem:                 "/etc/hysteria/key.pem",
		},
	})
	startInstance(t, option.Options{
		Log: &option.LogOptions{
			Level: "trace",
		},
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
				Type: C.TypeHysteria,
				HysteriaOptions: option.HysteriaOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					UpMbps:     100,
					DownMbps:   100,
					AuthString: "password",
					Obfs:       "fuck me till the daylight",
					TLS: &option.OutboundTLSOptions{
						Enabled:         true,
						ServerName:      "example.org",
						CertificatePath: certPem,
					},
				},
			},
		},
	})
	testSuitHy(t, clientPort, testPort)
}
